package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCValidatorRefreshesJWKSForRotatedKey(t *testing.T) {
	initial, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.RWMutex
	activeKey, activeKID := &initial.PublicKey, "initial"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		if r.URL.Path == "/.well-known/openid-configuration" {
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": "http://" + r.Host, "jwks_uri": "http://" + r.Host + "/keys"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kid": activeKID, "kty": "RSA", "n": base64.RawURLEncoding.EncodeToString(activeKey.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(activeKey.E)).Bytes())}}})
	}))
	defer server.Close()
	validator, err := newOIDCValidator(server.URL, "storemesh-bff")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	activeKey, activeKID = &rotated.PublicKey, "rotated"
	mu.Unlock()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &oidcClaims{RegisteredClaims: jwt.RegisteredClaims{Issuer: server.URL, Subject: "customer-1", Audience: jwt.ClaimStrings{"storemesh-bff"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}})
	token.Header["kid"] = "rotated"
	raw, err := token.SignedString(rotated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validator.Validate(raw); err != nil {
		t.Fatalf("validator did not accept rotated signing key: %v", err)
	}
}
