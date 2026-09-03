package authn

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const minJWTSecretSize = 32

var (
	ErrInvalidToken = errors.New("authn: invalid token")
	ErrExpiredToken = errors.New("authn: token expired")
)

type Claims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	Kind      string `json:"kind"`
	SessionID string `json:"sid,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	TokenID   string `json:"jti"`
}

type TokenManager struct {
	secret   []byte
	issuer   string
	audience string
	now      func() time.Time
}

func NewTokenManager(secret []byte, issuer, audience string) (*TokenManager, error) {
	if len(secret) < minJWTSecretSize {
		return nil, fmt.Errorf("authn: JWT secret must be at least %d bytes", minJWTSecretSize)
	}
	if issuer == "" || audience == "" {
		return nil, errors.New("authn: issuer and audience are required")
	}

	return &TokenManager{
		secret:   append([]byte(nil), secret...),
		issuer:   issuer,
		audience: audience,
		now:      time.Now,
	}, nil
}

func (m *TokenManager) Issue(subject, kind, sessionID string, ttl time.Duration) (string, time.Time, error) {
	if subject == "" || kind == "" || ttl <= 0 {
		return "", time.Time{}, ErrInvalidToken
	}

	now := m.now().UTC()
	expiresAt := now.Add(ttl)
	jti, err := RandomHex(16)
	if err != nil {
		return "", time.Time{}, err
	}

	claims := Claims{
		Issuer:    m.issuer,
		Audience:  m.audience,
		Subject:   subject,
		Kind:      kind,
		SessionID: sessionID,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		TokenID:   jti,
	}

	header, _ := json.Marshal(struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}{Algorithm: "HS256", Type: "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal JWT claims: %w", err)
	}

	unsigned := rawURL(header) + "." + rawURL(payload)
	signature := signHMAC(m.secret, unsigned)
	return unsigned + "." + rawURL(signature), expiresAt, nil
}

func (m *TokenManager) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != sha256.Size {
		return Claims{}, ErrInvalidToken
	}
	expected := signHMAC(m.secret, unsigned)
	if subtle.ConstantTimeCompare(signature, expected) != 1 {
		return Claims{}, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}

	now := m.now().UTC().Unix()
	if claims.Issuer != m.issuer || claims.Audience != m.audience || claims.Subject == "" || claims.Kind == "" || claims.TokenID == "" {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt <= now {
		return Claims{}, ErrExpiredToken
	}
	if claims.IssuedAt <= 0 || claims.IssuedAt > now+60 {
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}

func RandomToken(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("authn: invalid random token size")
	}
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func RandomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random value: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func TokenHash(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

func SecretMatchesHash(secret string, expected []byte) bool {
	hash := sha256.Sum256([]byte(secret))
	return subtle.ConstantTimeCompare(hash[:], expected) == 1
}

func signHMAC(secret []byte, value string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func rawURL(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
