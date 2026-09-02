package ethereum

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/hdwallet"
)

func signTransaction(privateKey []byte, tx domain.EthereumTransaction) (portout.TransactionSignature, error) {
	switch tx.Type {
	case domain.TransactionTypeLegacy:
		return signLegacyTransaction(privateKey, tx)
	case domain.TransactionTypeEIP1559:
		return signEIP1559Transaction(privateKey, tx)
	default:
		return portout.TransactionSignature{}, fmt.Errorf("%w: unsupported type %q", ErrInvalidTransaction, tx.Type)
	}
}

func signLegacyTransaction(privateKey []byte, tx domain.EthereumTransaction) (portout.TransactionSignature, error) {
	if tx.ChainID == 0 {
		return portout.TransactionSignature{}, fmt.Errorf("%w: chain_id is required", ErrInvalidTransaction)
	}

	gasPrice, err := parseQuantity(tx.GasPrice)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: gas_price: %v", ErrInvalidTransaction, err)
	}
	value, err := parseQuantity(tx.Value)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: value: %v", ErrInvalidTransaction, err)
	}
	to, err := decodeAddress(tx.To, true)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: to: %v", ErrInvalidTransaction, err)
	}
	data, err := decodeHex(tx.Data)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: data: %v", ErrInvalidTransaction, err)
	}

	unsigned := rlpList(
		rlpUint64(tx.Nonce),
		rlpBig(gasPrice),
		rlpUint64(tx.GasLimit),
		rlpBytes(to),
		rlpBig(value),
		rlpBytes(data),
		rlpUint64(tx.ChainID),
		rlpBytes(nil),
		rlpBytes(nil),
	)
	digest := keccak256(unsigned)
	sig, err := hdwallet.SignDigest(privateKey, digest[:])
	if err != nil {
		return portout.TransactionSignature{}, err
	}

	chainID := new(big.Int).SetUint64(tx.ChainID)
	v := new(big.Int).Mul(chainID, big.NewInt(2))
	v.Add(v, big.NewInt(35+int64(sig.RecoveryID)))

	raw := rlpList(
		rlpUint64(tx.Nonce),
		rlpBig(gasPrice),
		rlpUint64(tx.GasLimit),
		rlpBytes(to),
		rlpBig(value),
		rlpBytes(data),
		rlpBig(v),
		rlpBig(new(big.Int).SetBytes(sig.R[:])),
		rlpBig(new(big.Int).SetBytes(sig.S[:])),
	)
	hash := keccak256(raw)

	return transactionSignature(raw, hash, sig), nil
}

func signEIP1559Transaction(privateKey []byte, tx domain.EthereumTransaction) (portout.TransactionSignature, error) {
	if tx.ChainID == 0 {
		return portout.TransactionSignature{}, fmt.Errorf("%w: chain_id is required", ErrInvalidTransaction)
	}

	priorityFee, err := parseQuantity(tx.MaxPriorityFeePerGas)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: max_priority_fee_per_gas: %v", ErrInvalidTransaction, err)
	}
	maxFee, err := parseQuantity(tx.MaxFeePerGas)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: max_fee_per_gas: %v", ErrInvalidTransaction, err)
	}
	if priorityFee.Cmp(maxFee) > 0 {
		return portout.TransactionSignature{}, fmt.Errorf("%w: max_priority_fee_per_gas exceeds max_fee_per_gas", ErrInvalidTransaction)
	}
	value, err := parseQuantity(tx.Value)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: value: %v", ErrInvalidTransaction, err)
	}
	to, err := decodeAddress(tx.To, true)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: to: %v", ErrInvalidTransaction, err)
	}
	data, err := decodeHex(tx.Data)
	if err != nil {
		return portout.TransactionSignature{}, fmt.Errorf("%w: data: %v", ErrInvalidTransaction, err)
	}
	accessList, err := encodeAccessList(tx.AccessList)
	if err != nil {
		return portout.TransactionSignature{}, err
	}

	unsignedPayload := rlpList(
		rlpUint64(tx.ChainID),
		rlpUint64(tx.Nonce),
		rlpBig(priorityFee),
		rlpBig(maxFee),
		rlpUint64(tx.GasLimit),
		rlpBytes(to),
		rlpBig(value),
		rlpBytes(data),
		accessList,
	)
	unsigned := append([]byte{0x02}, unsignedPayload...)
	digest := keccak256(unsigned)
	sig, err := hdwallet.SignDigest(privateKey, digest[:])
	if err != nil {
		return portout.TransactionSignature{}, err
	}

	signedPayload := rlpList(
		rlpUint64(tx.ChainID),
		rlpUint64(tx.Nonce),
		rlpBig(priorityFee),
		rlpBig(maxFee),
		rlpUint64(tx.GasLimit),
		rlpBytes(to),
		rlpBig(value),
		rlpBytes(data),
		accessList,
		rlpUint64(uint64(sig.RecoveryID)),
		rlpBig(new(big.Int).SetBytes(sig.R[:])),
		rlpBig(new(big.Int).SetBytes(sig.S[:])),
	)
	raw := append([]byte{0x02}, signedPayload...)
	hash := keccak256(raw)
	return transactionSignature(raw, hash, sig), nil
}

func verifyTransaction(publicKey, raw []byte) (bool, error) {
	if len(raw) == 0 {
		return false, fmt.Errorf("%w: empty raw transaction", ErrInvalidTransaction)
	}
	if raw[0] == 0x02 {
		return verifyEIP1559Transaction(publicKey, raw)
	}
	return verifyLegacyTransaction(publicKey, raw)
}

func verifyLegacyTransaction(publicKey, raw []byte) (bool, error) {
	node, consumed, err := decodeRLP(raw)
	if err != nil || consumed != len(raw) || !node.isList || len(node.list) != 9 {
		return false, fmt.Errorf("%w: malformed legacy transaction", ErrInvalidTransaction)
	}

	v, err := node.list[6].integer()
	if err != nil {
		return false, fmt.Errorf("%w: invalid v", ErrInvalidTransaction)
	}
	r, s, err := signatureScalars(node.list[7], node.list[8])
	if err != nil {
		return false, err
	}

	var (
		unsigned []byte
		parity   byte
	)
	switch {
	case v.Cmp(big.NewInt(27)) == 0 || v.Cmp(big.NewInt(28)) == 0:
		parity = byte(new(big.Int).Sub(v, big.NewInt(27)).Uint64())
		unsigned = rlpListFromNodes(node.list[:6])
	case v.Cmp(big.NewInt(35)) >= 0:
		t := new(big.Int).Sub(v, big.NewInt(35))
		parityValue := new(big.Int).And(new(big.Int).Set(t), big.NewInt(1))
		parity = byte(parityValue.Uint64())
		chainID := new(big.Int).Sub(t, parityValue)
		chainID.Div(chainID, big.NewInt(2))
		if chainID.Sign() <= 0 {
			return false, fmt.Errorf("%w: invalid chain id", ErrInvalidTransaction)
		}
		unsigned = rlpList(
			node.list[0].encode(), node.list[1].encode(), node.list[2].encode(),
			node.list[3].encode(), node.list[4].encode(), node.list[5].encode(),
			rlpBig(chainID), rlpBytes(nil), rlpBytes(nil),
		)
	default:
		return false, fmt.Errorf("%w: invalid v", ErrInvalidTransaction)
	}

	digest := keccak256(unsigned)
	sig := append(r, s...)
	return hdwallet.VerifyDigestWithRecoveryID(publicKey, digest[:], sig, parity), nil
}

func verifyEIP1559Transaction(publicKey, raw []byte) (bool, error) {
	node, consumed, err := decodeRLP(raw[1:])
	if err != nil || consumed != len(raw)-1 || !node.isList || len(node.list) != 12 {
		return false, fmt.Errorf("%w: malformed EIP-1559 transaction", ErrInvalidTransaction)
	}
	parity, err := node.list[9].uint64()
	if err != nil || parity > 1 {
		return false, fmt.Errorf("%w: invalid y parity", ErrInvalidTransaction)
	}
	r, s, err := signatureScalars(node.list[10], node.list[11])
	if err != nil {
		return false, err
	}

	unsignedPayload := rlpListFromNodes(node.list[:9])
	unsigned := append([]byte{0x02}, unsignedPayload...)
	digest := keccak256(unsigned)
	sig := append(r, s...)
	return hdwallet.VerifyDigestWithRecoveryID(publicKey, digest[:], sig, byte(parity)), nil
}

func transactionSignature(raw []byte, hash [32]byte, sig hdwallet.Signature) portout.TransactionSignature {
	return portout.TransactionSignature{
		RawTransaction: append([]byte(nil), raw...),
		Hash:           append([]byte(nil), hash[:]...),
		R:              append([]byte(nil), sig.R[:]...),
		S:              append([]byte(nil), sig.S[:]...),
		YParity:        sig.RecoveryID,
	}
}

func signatureScalars(rNode, sNode rlpNode) ([]byte, []byte, error) {
	rValue, err := rNode.integer()
	if err != nil || rValue.Sign() <= 0 || rValue.BitLen() > 256 {
		return nil, nil, fmt.Errorf("%w: invalid r", ErrInvalidSignature)
	}
	sValue, err := sNode.integer()
	if err != nil || sValue.Sign() <= 0 || sValue.BitLen() > 256 {
		return nil, nil, fmt.Errorf("%w: invalid s", ErrInvalidSignature)
	}
	r := rValue.FillBytes(make([]byte, 32))
	s := sValue.FillBytes(make([]byte, 32))
	if _, err := hdwallet.ParseCompactSignature(append(append([]byte(nil), r...), s...)); err != nil {
		return nil, nil, ErrInvalidSignature
	}
	return r, s, nil
}

func encodeAccessList(entries []domain.AccessListEntry) ([]byte, error) {
	items := make([][]byte, 0, len(entries))
	for _, entry := range entries {
		address, err := decodeAddress(entry.Address, false)
		if err != nil {
			return nil, fmt.Errorf("%w: access list address: %v", ErrInvalidTransaction, err)
		}
		keys := make([][]byte, 0, len(entry.StorageKeys))
		for _, value := range entry.StorageKeys {
			key, err := decodeFixedHex(value, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: access list storage key: %v", ErrInvalidTransaction, err)
			}
			keys = append(keys, rlpBytes(key))
		}
		items = append(items, rlpList(rlpBytes(address), rlpList(keys...)))
	}
	return rlpList(items...), nil
}

func parseQuantity(value string) (*big.Int, error) {
	if value == "" {
		return new(big.Int), nil
	}
	base := 10
	text := value
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 16
		text = value[2:]
		if text == "" {
			return nil, errors.New("empty hexadecimal quantity")
		}
	}
	n, ok := new(big.Int).SetString(text, base)
	if !ok || n.Sign() < 0 || n.BitLen() > 256 {
		return nil, errors.New("quantity must be an unsigned 256-bit integer")
	}
	return n, nil
}

func decodeAddress(value string, allowEmpty bool) ([]byte, error) {
	if value == "" && allowEmpty {
		return nil, nil
	}
	return decodeFixedHex(value, 20)
}

func decodeFixedHex(value string, size int) ([]byte, error) {
	decoded, err := decodeHex(value)
	if err != nil {
		return nil, err
	}
	if len(decoded) != size {
		return nil, fmt.Errorf("expected %d bytes", size)
	}
	return decoded, nil
}

func decodeHex(value string) ([]byte, error) {
	if value == "" || value == "0x" || value == "0X" {
		return nil, nil
	}
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		value = value[2:]
	}
	if len(value)%2 != 0 {
		return nil, errors.New("hex value must have even length")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, errors.New("invalid hexadecimal value")
	}
	return decoded, nil
}

type rlpNode struct {
	bytes  []byte
	list   []rlpNode
	isList bool
}

func (n rlpNode) encode() []byte {
	if n.isList {
		parts := make([][]byte, len(n.list))
		for i := range n.list {
			parts[i] = n.list[i].encode()
		}
		return rlpList(parts...)
	}
	return rlpBytes(n.bytes)
}

func (n rlpNode) integer() (*big.Int, error) {
	if n.isList {
		return nil, errors.New("RLP list is not an integer")
	}
	if len(n.bytes) > 0 && n.bytes[0] == 0 {
		return nil, errors.New("non-canonical integer")
	}
	return new(big.Int).SetBytes(n.bytes), nil
}

func (n rlpNode) uint64() (uint64, error) {
	value, err := n.integer()
	if err != nil || !value.IsUint64() {
		return 0, errors.New("integer exceeds uint64")
	}
	return value.Uint64(), nil
}

func rlpListFromNodes(nodes []rlpNode) []byte {
	parts := make([][]byte, len(nodes))
	for i := range nodes {
		parts[i] = nodes[i].encode()
	}
	return rlpList(parts...)
}

func rlpUint64(value uint64) []byte {
	return rlpBig(new(big.Int).SetUint64(value))
}

func rlpBig(value *big.Int) []byte {
	if value == nil || value.Sign() == 0 {
		return rlpBytes(nil)
	}
	return rlpBytes(value.Bytes())
}

func rlpBytes(value []byte) []byte {
	if len(value) == 1 && value[0] < 0x80 {
		return append([]byte(nil), value...)
	}
	if len(value) <= 55 {
		out := make([]byte, 1+len(value))
		out[0] = byte(0x80 + len(value))
		copy(out[1:], value)
		return out
	}
	length := intBytes(len(value))
	out := make([]byte, 1+len(length)+len(value))
	out[0] = byte(0xb7 + len(length))
	copy(out[1:], length)
	copy(out[1+len(length):], value)
	return out
}

func rlpList(items ...[]byte) []byte {
	payload := bytes.Join(items, nil)
	if len(payload) <= 55 {
		return append([]byte{byte(0xc0 + len(payload))}, payload...)
	}
	length := intBytes(len(payload))
	out := make([]byte, 1+len(length)+len(payload))
	out[0] = byte(0xf7 + len(length))
	copy(out[1:], length)
	copy(out[1+len(length):], payload)
	return out
}

func intBytes(value int) []byte {
	if value == 0 {
		return nil
	}
	var buf [8]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte(value)
		value >>= 8
	}
	return append([]byte(nil), buf[i:]...)
}

func decodeRLP(data []byte) (rlpNode, int, error) {
	if len(data) == 0 {
		return rlpNode{}, 0, errors.New("empty RLP")
	}
	prefix := data[0]
	switch {
	case prefix <= 0x7f:
		return rlpNode{bytes: []byte{prefix}}, 1, nil
	case prefix <= 0xb7:
		length := int(prefix - 0x80)
		if len(data) < 1+length {
			return rlpNode{}, 0, errors.New("short RLP string")
		}
		value := data[1 : 1+length]
		if length == 1 && value[0] < 0x80 {
			return rlpNode{}, 0, errors.New("non-canonical RLP string")
		}
		return rlpNode{bytes: append([]byte(nil), value...)}, 1 + length, nil
	case prefix <= 0xbf:
		lengthOfLength := int(prefix - 0xb7)
		length, offset, err := decodeRLPLength(data, lengthOfLength)
		if err != nil || length <= 55 || len(data) < offset+length {
			return rlpNode{}, 0, errors.New("invalid long RLP string")
		}
		return rlpNode{bytes: append([]byte(nil), data[offset:offset+length]...)}, offset + length, nil
	case prefix <= 0xf7:
		length := int(prefix - 0xc0)
		if len(data) < 1+length {
			return rlpNode{}, 0, errors.New("short RLP list")
		}
		items, err := decodeRLPList(data[1 : 1+length])
		if err != nil {
			return rlpNode{}, 0, err
		}
		return rlpNode{list: items, isList: true}, 1 + length, nil
	default:
		lengthOfLength := int(prefix - 0xf7)
		length, offset, err := decodeRLPLength(data, lengthOfLength)
		if err != nil || length <= 55 || len(data) < offset+length {
			return rlpNode{}, 0, errors.New("invalid long RLP list")
		}
		items, err := decodeRLPList(data[offset : offset+length])
		if err != nil {
			return rlpNode{}, 0, err
		}
		return rlpNode{list: items, isList: true}, offset + length, nil
	}
}

func decodeRLPLength(data []byte, lengthOfLength int) (int, int, error) {
	if lengthOfLength <= 0 || lengthOfLength > 8 || len(data) < 1+lengthOfLength || data[1] == 0 {
		return 0, 0, errors.New("invalid RLP length")
	}
	length := 0
	for _, b := range data[1 : 1+lengthOfLength] {
		if length > (int(^uint(0)>>1)-int(b))/256 {
			return 0, 0, errors.New("RLP length overflow")
		}
		length = length*256 + int(b)
	}
	return length, 1 + lengthOfLength, nil
}

func decodeRLPList(payload []byte) ([]rlpNode, error) {
	var nodes []rlpNode
	for len(payload) > 0 {
		node, consumed, err := decodeRLP(payload)
		if err != nil || consumed <= 0 || consumed > len(payload) {
			return nil, errors.New("invalid RLP list payload")
		}
		nodes = append(nodes, node)
		payload = payload[consumed:]
	}
	return nodes, nil
}
