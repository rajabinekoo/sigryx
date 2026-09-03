package authn

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("password hash contains plaintext password")
	}

	valid, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !valid {
		t.Fatalf("expected password to verify, valid=%v err=%v", valid, err)
	}
	valid, err = VerifyPassword(hash, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("wrong password verified")
	}
}

func TestTokenManagerIssueVerifyAndRejectTampering(t *testing.T) {
	manager, err := NewTokenManager([]byte(strings.Repeat("x", 32)), "sigryx", "sigryx-api")
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Unix(1_800_000_000, 0)
	manager.now = func() time.Time { return fixed }

	token, _, err := manager.Issue("user-1", "USER", "session-1", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || claims.SessionID != "session-1" || claims.Kind != "USER" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	parts := strings.Split(token, ".")
	replacement := byte('A')
	if parts[1][len(parts[1])-1] == replacement {
		replacement = 'B'
	}
	parts[1] = parts[1][:len(parts[1])-1] + string(replacement)
	if _, err := manager.Verify(strings.Join(parts, ".")); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token, got %v", err)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager, err := NewTokenManager([]byte(strings.Repeat("x", 32)), "sigryx", "sigryx-api")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	manager.now = func() time.Time { return now }
	token, _, err := manager.Issue("user-1", "USER", "session-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := manager.Verify(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected expired token, got %v", err)
	}
}
