package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokensRequireAlgorithmIssuerExpiryAndIdentity(t *testing.T) {
	const secret = "private-fixture-jwt-secret-at-least-32-bytes"
	valid, err := GenerateToken(1, "judge", "admin", secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(valid, secret); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"algorithm", "issuer", "missing-expiry", "expired", "zero-user", "empty-name", "unknown-role", "wrong-key"} {
		t.Run(kind, func(t *testing.T) {
			claims := Claims{UserID: 1, Username: "judge", Role: "admin", RegisteredClaims: jwt.RegisteredClaims{Issuer: "agentrix-server", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
			method, key := jwt.SigningMethodHS256, secret
			switch kind {
			case "algorithm":
				method = jwt.SigningMethodHS384
			case "issuer":
				claims.Issuer = "other-server"
			case "missing-expiry":
				claims.ExpiresAt = nil
			case "expired":
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
			case "zero-user":
				claims.UserID = 0
			case "empty-name":
				claims.Username = ""
			case "unknown-role":
				claims.Role = "superuser"
			case "wrong-key":
				key = "another-private-fixture-signing-key"
			}
			token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateToken(token, secret); err == nil {
				t.Fatal("accepted invalid token")
			}
		})
	}
}
