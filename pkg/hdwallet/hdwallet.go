// Package hdwallet implements blockchain-agnostic BIP32 private-key derivation.
package hdwallet

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

const hardenedOffset uint32 = 1 << 31

var (
	ErrInvalidSeed       = errors.New("hdwallet: invalid seed")
	ErrInvalidPath       = errors.New("hdwallet: invalid derivation path")
	ErrInvalidPrivateKey = errors.New("hdwallet: invalid private key")
	ErrInvalidChildKey   = errors.New("hdwallet: invalid child key")
	ErrNilCallback       = errors.New("hdwallet: callback is nil")
)

type Segment struct {
	Index    uint32
	Hardened bool
}

type Path struct {
	segments []Segment
}

func NewPath(segments ...Segment) (Path, error) {
	for _, segment := range segments {
		if segment.Index >= hardenedOffset {
			return Path{}, ErrInvalidPath
		}
	}

	return Path{segments: append([]Segment(nil), segments...)}, nil
}

func BIP44(coinType, account, change, index uint32) (Path, error) {
	return NewPath(
		Segment{Index: 44, Hardened: true},
		Segment{Index: coinType, Hardened: true},
		Segment{Index: account, Hardened: true},
		Segment{Index: change},
		Segment{Index: index},
	)
}

func (p Path) Segments() []Segment {
	return append([]Segment(nil), p.segments...)
}

func (p Path) String() string {
	result := "m"
	for _, segment := range p.segments {
		if segment.Hardened {
			result += fmt.Sprintf("/%d'", segment.Index)
			continue
		}
		result += fmt.Sprintf("/%d", segment.Index)
	}
	return result
}

// DerivePrivateKey derives a BIP32 secp256k1 private key and exposes it only
// for the duration of fn. The derived key is wiped before the function returns.
func DerivePrivateKey(seed []byte, path Path, fn func([]byte) error) error {
	if len(seed) < 16 || len(seed) > 64 {
		return ErrInvalidSeed
	}
	if fn == nil {
		return ErrNilCallback
	}

	key, chainCode, err := masterKey(seed)
	if err != nil {
		return err
	}
	defer func() {
		clear(key[:])
		clear(chainCode[:])
	}()

	for _, segment := range path.segments {
		nextKey, nextChainCode, deriveErr := deriveChild(key, chainCode, segment)
		clear(key[:])
		clear(chainCode[:])
		if deriveErr != nil {
			return deriveErr
		}
		key = nextKey
		chainCode = nextChainCode
	}

	return fn(key[:])
}

// UncompressedPublicKey returns the canonical 65-byte secp256k1 public key.
func UncompressedPublicKey(privateKey []byte) ([]byte, error) {
	point, err := publicPoint(privateKey)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 65)
	out[0] = 0x04
	point.x.FillBytes(out[1:33])
	point.y.FillBytes(out[33:65])
	return out, nil
}

func masterKey(seed []byte) ([32]byte, [32]byte, error) {
	mac := hmac.New(sha512.New, []byte("Bitcoin seed"))
	_, _ = mac.Write(seed)
	sum := mac.Sum(nil)
	defer clear(sum)

	var key [32]byte
	var chainCode [32]byte
	copy(key[:], sum[:32])
	copy(chainCode[:], sum[32:])

	if !validPrivateKey(key[:]) {
		clear(key[:])
		clear(chainCode[:])
		return [32]byte{}, [32]byte{}, ErrInvalidPrivateKey
	}

	return key, chainCode, nil
}

func deriveChild(
	parentKey [32]byte,
	parentChainCode [32]byte,
	segment Segment,
) ([32]byte, [32]byte, error) {
	defer clear(parentKey[:])
	defer clear(parentChainCode[:])
	if segment.Index >= hardenedOffset {
		return [32]byte{}, [32]byte{}, ErrInvalidPath
	}

	index := segment.Index
	data := make([]byte, 37)
	defer clear(data)

	if segment.Hardened {
		index += hardenedOffset
		data[0] = 0x00
		copy(data[1:33], parentKey[:])
	} else {
		publicKey, err := compressedPublicKey(parentKey[:])
		if err != nil {
			return [32]byte{}, [32]byte{}, err
		}
		copy(data[:33], publicKey)
		clear(publicKey)
	}

	binary.BigEndian.PutUint32(data[33:], index)

	mac := hmac.New(sha512.New, parentChainCode[:])
	_, _ = mac.Write(data)
	sum := mac.Sum(nil)
	defer clear(sum)

	var left [32]byte
	copy(left[:], sum[:32])
	defer clear(left[:])

	if !validPrivateKey(left[:]) {
		return [32]byte{}, [32]byte{}, ErrInvalidChildKey
	}

	childKey := addScalarsModN(left, parentKey)
	if !validPrivateKey(childKey[:]) {
		clear(childKey[:])
		return [32]byte{}, [32]byte{}, ErrInvalidChildKey
	}

	var childChainCode [32]byte
	copy(childChainCode[:], sum[32:])
	return childKey, childChainCode, nil
}

func compressedPublicKey(privateKey []byte) ([]byte, error) {
	point, err := publicPoint(privateKey)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 33)
	out[0] = 0x02
	if point.y.Bit(0) == 1 {
		out[0] = 0x03
	}
	point.x.FillBytes(out[1:])
	return out, nil
}

func validPrivateKey(privateKey []byte) bool {
	if len(privateKey) != 32 {
		return false
	}

	nonZero := false
	for _, b := range privateKey {
		if b != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		return false
	}

	for i := range privateKey {
		if privateKey[i] < curveNBytes[i] {
			return true
		}
		if privateKey[i] > curveNBytes[i] {
			return false
		}
	}

	return false
}

func addScalarsModN(a, b [32]byte) [32]byte {
	var sum [33]byte
	carry := 0

	for i := 31; i >= 0; i-- {
		v := int(a[i]) + int(b[i]) + carry
		sum[i+1] = byte(v)
		carry = v >> 8
	}
	sum[0] = byte(carry)

	if scalar33AtLeastN(sum) {
		borrow := 0
		for i := 32; i >= 1; i-- {
			v := int(sum[i]) - int(curveNBytes[i-1]) - borrow
			if v < 0 {
				v += 256
				borrow = 1
			} else {
				borrow = 0
			}
			sum[i] = byte(v)
		}
		sum[0] = byte(int(sum[0]) - borrow)
	}

	var result [32]byte
	copy(result[:], sum[1:])
	clear(sum[:])
	return result
}

func scalar33AtLeastN(value [33]byte) bool {
	if value[0] != 0 {
		return true
	}

	for i := 0; i < 32; i++ {
		if value[i+1] > curveNBytes[i] {
			return true
		}
		if value[i+1] < curveNBytes[i] {
			return false
		}
	}

	return true
}

type curvePoint struct {
	x        *big.Int
	y        *big.Int
	infinity bool
}

var curveNBytes = [32]byte{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe,
	0xba, 0xae, 0xdc, 0xe6, 0xaf, 0x48, 0xa0, 0x3b,
	0xbf, 0xd2, 0x5e, 0x8c, 0xd0, 0x36, 0x41, 0x41,
}

var (
	curveP  = mustBig("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F")
	curveGx = mustBig("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
	curveGy = mustBig("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8")
)

func mustBig(value string) *big.Int {
	n, ok := new(big.Int).SetString(value, 16)
	if !ok {
		panic("invalid secp256k1 constant")
	}
	return n
}

func publicPoint(privateKey []byte) (curvePoint, error) {
	if !validPrivateKey(privateKey) {
		return curvePoint{}, ErrInvalidPrivateKey
	}
	return scalarBaseMult(privateKey), nil
}

func scalarBaseMult(scalar []byte) curvePoint {
	result := curvePoint{infinity: true}
	base := curvePoint{x: new(big.Int).Set(curveGx), y: new(big.Int).Set(curveGy)}

	for _, b := range scalar {
		for bit := 7; bit >= 0; bit-- {
			result = pointDouble(result)
			if b&(1<<uint(bit)) != 0 {
				result = pointAdd(result, base)
			}
		}
	}

	return result
}

func pointAdd(a, b curvePoint) curvePoint {
	if a.infinity {
		return clonePoint(b)
	}
	if b.infinity {
		return clonePoint(a)
	}

	if a.x.Cmp(b.x) == 0 {
		ysum := new(big.Int).Add(a.y, b.y)
		ysum.Mod(ysum, curveP)
		if ysum.Sign() == 0 {
			return curvePoint{infinity: true}
		}
		return pointDouble(a)
	}

	numerator := new(big.Int).Sub(b.y, a.y)
	numerator.Mod(numerator, curveP)
	denominator := new(big.Int).Sub(b.x, a.x)
	denominator.Mod(denominator, curveP)
	inverse := new(big.Int).ModInverse(denominator, curveP)
	if inverse == nil {
		return curvePoint{infinity: true}
	}

	lambda := new(big.Int).Mul(numerator, inverse)
	lambda.Mod(lambda, curveP)

	x := new(big.Int).Mul(lambda, lambda)
	x.Sub(x, a.x)
	x.Sub(x, b.x)
	x.Mod(x, curveP)

	y := new(big.Int).Sub(a.x, x)
	y.Mul(lambda, y)
	y.Sub(y, a.y)
	y.Mod(y, curveP)

	return curvePoint{x: x, y: y}
}

func pointDouble(a curvePoint) curvePoint {
	if a.infinity || a.y.Sign() == 0 {
		return curvePoint{infinity: true}
	}

	numerator := new(big.Int).Mul(a.x, a.x)
	numerator.Mul(numerator, big.NewInt(3))
	numerator.Mod(numerator, curveP)

	denominator := new(big.Int).Lsh(new(big.Int).Set(a.y), 1)
	denominator.Mod(denominator, curveP)
	inverse := new(big.Int).ModInverse(denominator, curveP)
	if inverse == nil {
		return curvePoint{infinity: true}
	}

	lambda := new(big.Int).Mul(numerator, inverse)
	lambda.Mod(lambda, curveP)

	x := new(big.Int).Mul(lambda, lambda)
	twoX := new(big.Int).Lsh(new(big.Int).Set(a.x), 1)
	x.Sub(x, twoX)
	x.Mod(x, curveP)

	y := new(big.Int).Sub(a.x, x)
	y.Mul(lambda, y)
	y.Sub(y, a.y)
	y.Mod(y, curveP)

	return curvePoint{x: x, y: y}
}

func clonePoint(p curvePoint) curvePoint {
	if p.infinity {
		return curvePoint{infinity: true}
	}
	return curvePoint{x: new(big.Int).Set(p.x), y: new(big.Int).Set(p.y)}
}
