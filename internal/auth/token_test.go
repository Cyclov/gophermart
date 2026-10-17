package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokens(t *testing.T) {
	secret := strings.Repeat("s", 32)
	tokens, err := New(secret)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := tokens.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	id, err := tokens.Parse(raw)
	if err != nil || id != 42 {
		t.Fatal("token round trip failed")
	}
	other, _ := New(strings.Repeat("x", 32))
	if _, err := other.Parse(raw); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := New("short"); err == nil {
		t.Fatal("short key accepted")
	}
	for _, tt := range []struct {
		name   string
		method jwt.SigningMethod
		claims jwt.RegisteredClaims
	}{
		{"expired", jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "42", Issuer: "gophermart", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}},
		{"no expiry", jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "42", Issuer: "gophermart"}},
		{"wrong issuer", jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "42", Issuer: "other", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		{"invalid subject", jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "0", Issuer: "gophermart", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		{"wrong algorithm", jwt.SigningMethodHS384, jwt.RegisteredClaims{Subject: "42", Issuer: "gophermart", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tt.method, tt.claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tokens.Parse(token); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	parts := strings.Split(raw, ".")
	parts[1] = "eyJzdWIiOiI5OSJ9"
	if _, err := tokens.Parse(strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered token accepted")
	}
}
