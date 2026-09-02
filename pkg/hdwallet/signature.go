package hdwallet

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"math/big"
)

var (
	ErrInvalidDigest    = errors.New("hdwallet: digest must be 32 bytes")
	ErrInvalidSignature = errors.New("hdwallet: invalid secp256k1 signature")
	ErrInvalidPublicKey = errors.New("hdwallet: invalid secp256k1 public key")
)

type Signature struct {
	R          [32]byte
	S          [32]byte
	RecoveryID byte
}

func (s Signature) Compact() []byte {
	out := make([]byte, 64)
	copy(out[:32], s.R[:])
	copy(out[32:], s.S[:])
	return out
}

func (s Signature) Ethereum() []byte {
	out := make([]byte, 65)
	copy(out[:32], s.R[:])
	copy(out[32:64], s.S[:])
	out[64] = s.RecoveryID + 27
	return out
}

func ParseCompactSignature(raw []byte) (Signature, error) {
	if len(raw) != 64 {
		return Signature{}, ErrInvalidSignature
	}

	var sig Signature
	copy(sig.R[:], raw[:32])
	copy(sig.S[:], raw[32:])
	if !validSignatureScalar(sig.R[:]) || !validSignatureScalar(sig.S[:]) {
		return Signature{}, ErrInvalidSignature
	}
	return sig, nil
}

func ParseEthereumSignature(raw []byte) (Signature, error) {
	if len(raw) != 65 {
		return Signature{}, ErrInvalidSignature
	}

	sig, err := ParseCompactSignature(raw[:64])
	if err != nil {
		return Signature{}, err
	}

	switch raw[64] {
	case 0, 1:
		sig.RecoveryID = raw[64]
	case 27, 28:
		sig.RecoveryID = raw[64] - 27
	default:
		return Signature{}, ErrInvalidSignature
	}
	return sig, nil
}

// SignDigest returns a deterministic RFC6979 ECDSA signature with low-S
// normalization. RecoveryID is the Ethereum y-parity bit (0 or 1).
func SignDigest(privateKey, digest []byte) (Signature, error) {
	if !validPrivateKey(privateKey) {
		return Signature{}, ErrInvalidPrivateKey
	}
	if len(digest) != 32 {
		return Signature{}, ErrInvalidDigest
	}

	d := new(big.Int).SetBytes(privateKey)
	z := new(big.Int).SetBytes(digest)
	z.Mod(z, curveN())

	nonce := newRFC6979(privateKey, digest)
	defer nonce.destroy()
	for {
		k := nonce.next()
		point := scalarBaseMult(k.FillBytes(make([]byte, 32)))
		if point.infinity || point.x.Cmp(curveN()) >= 0 {
			nonce.reject()
			continue
		}

		r := new(big.Int).Set(point.x)
		if r.Sign() == 0 {
			nonce.reject()
			continue
		}

		kinv := new(big.Int).ModInverse(k, curveN())
		if kinv == nil {
			nonce.reject()
			continue
		}

		s := new(big.Int).Mul(r, d)
		s.Add(s, z)
		s.Mul(s, kinv)
		s.Mod(s, curveN())
		if s.Sign() == 0 {
			nonce.reject()
			continue
		}

		parity := byte(point.y.Bit(0))
		halfN := new(big.Int).Rsh(new(big.Int).Set(curveN()), 1)
		if s.Cmp(halfN) > 0 {
			s.Sub(curveN(), s)
			parity ^= 1
		}

		var result Signature
		r.FillBytes(result.R[:])
		s.FillBytes(result.S[:])
		result.RecoveryID = parity
		return result, nil
	}
}

func VerifyDigest(publicKey, digest, signature []byte) bool {
	sig, err := ParseCompactSignature(signature)
	if err != nil {
		return false
	}
	return verifySignature(publicKey, digest, sig, false)
}

// VerifyDigestWithRecoveryID verifies both the ECDSA signature and the
// Ethereum y-parity bit carried by transactions.
func VerifyDigestWithRecoveryID(
	publicKey, digest, signature []byte,
	recoveryID byte,
) bool {
	if recoveryID > 1 {
		return false
	}
	sig, err := ParseCompactSignature(signature)
	if err != nil {
		return false
	}
	sig.RecoveryID = recoveryID
	return verifySignature(publicKey, digest, sig, true)
}

// VerifyEthereumDigest verifies a 65-byte Ethereum signature (r || s || v),
// including its recovery/y-parity byte.
func VerifyEthereumDigest(publicKey, digest, signature []byte) bool {
	sig, err := ParseEthereumSignature(signature)
	if err != nil {
		return false
	}
	return verifySignature(publicKey, digest, sig, true)
}

func verifySignature(
	publicKey, digest []byte,
	sig Signature,
	verifyRecoveryID bool,
) bool {
	if len(digest) != 32 {
		return false
	}

	q, err := parsePublicKey(publicKey)
	if err != nil {
		return false
	}

	r := new(big.Int).SetBytes(sig.R[:])
	s := new(big.Int).SetBytes(sig.S[:])
	halfN := new(big.Int).Rsh(new(big.Int).Set(curveN()), 1)
	if s.Cmp(halfN) > 0 {
		return false
	}

	z := new(big.Int).SetBytes(digest)
	z.Mod(z, curveN())
	w := new(big.Int).ModInverse(s, curveN())
	if w == nil {
		return false
	}

	u1 := new(big.Int).Mul(z, w)
	u1.Mod(u1, curveN())
	u2 := new(big.Int).Mul(r, w)
	u2.Mod(u2, curveN())

	p1 := scalarBaseMult(u1.FillBytes(make([]byte, 32)))
	p2 := scalarMult(q, u2)
	point := pointAdd(p1, p2)
	if point.infinity {
		return false
	}

	x := new(big.Int).Mod(point.x, curveN())
	if x.Cmp(r) != 0 {
		return false
	}
	if verifyRecoveryID && byte(point.y.Bit(0)) != sig.RecoveryID {
		return false
	}
	return true
}

func curveN() *big.Int {
	return new(big.Int).SetBytes(curveNBytes[:])
}

func validSignatureScalar(value []byte) bool {
	if len(value) != 32 {
		return false
	}
	n := new(big.Int).SetBytes(value)
	return n.Sign() > 0 && n.Cmp(curveN()) < 0
}

func parsePublicKey(raw []byte) (curvePoint, error) {
	if len(raw) != 65 || raw[0] != 0x04 {
		return curvePoint{}, ErrInvalidPublicKey
	}
	x := new(big.Int).SetBytes(raw[1:33])
	y := new(big.Int).SetBytes(raw[33:65])
	if x.Cmp(curveP) >= 0 || y.Cmp(curveP) >= 0 || !isOnCurve(x, y) {
		return curvePoint{}, ErrInvalidPublicKey
	}
	return curvePoint{x: x, y: y}, nil
}

func isOnCurve(x, y *big.Int) bool {
	left := new(big.Int).Mul(y, y)
	left.Mod(left, curveP)

	right := new(big.Int).Mul(x, x)
	right.Mul(right, x)
	right.Add(right, big.NewInt(7))
	right.Mod(right, curveP)
	return left.Cmp(right) == 0
}

func scalarMult(point curvePoint, scalar *big.Int) curvePoint {
	result := curvePoint{infinity: true}
	base := clonePoint(point)
	bytes := scalar.FillBytes(make([]byte, 32))
	defer clear(bytes)

	for _, b := range bytes {
		for bit := 7; bit >= 0; bit-- {
			result = pointDouble(result)
			if b&(1<<uint(bit)) != 0 {
				result = pointAdd(result, base)
			}
		}
	}
	return result
}

type rfc6979 struct {
	k [32]byte
	v [32]byte
}

func newRFC6979(privateKey, digest []byte) *rfc6979 {
	g := &rfc6979{}
	for i := range g.v {
		g.v[i] = 0x01
	}

	x := append([]byte(nil), privateKey...)
	defer clear(x)
	h1 := new(big.Int).SetBytes(digest)
	h1.Mod(h1, curveN())
	h1bytes := h1.FillBytes(make([]byte, 32))
	defer clear(h1bytes)

	g.update(0x00, x, h1bytes)
	g.update(0x01, x, h1bytes)
	return g
}

func (g *rfc6979) update(marker byte, parts ...[]byte) {
	mac := hmac.New(sha256.New, g.k[:])
	_, _ = mac.Write(g.v[:])
	_, _ = mac.Write([]byte{marker})
	for _, part := range parts {
		_, _ = mac.Write(part)
	}
	copy(g.k[:], mac.Sum(nil))

	mac = hmac.New(sha256.New, g.k[:])
	_, _ = mac.Write(g.v[:])
	copy(g.v[:], mac.Sum(nil))
}

func (g *rfc6979) reject() {
	g.update(0x00)
}

func (g *rfc6979) destroy() {
	clear(g.k[:])
	clear(g.v[:])
}

func (g *rfc6979) next() *big.Int {
	for {
		mac := hmac.New(sha256.New, g.k[:])
		_, _ = mac.Write(g.v[:])
		copy(g.v[:], mac.Sum(nil))

		candidate := new(big.Int).SetBytes(g.v[:])
		if candidate.Sign() > 0 && candidate.Cmp(curveN()) < 0 {
			return candidate
		}

		g.update(0x00)
	}
}
