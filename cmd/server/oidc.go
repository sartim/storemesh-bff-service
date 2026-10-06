package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type oidcValidator struct {
	issuer, audience, jwksURL string
	keys                      map[string]*rsa.PublicKey
	mu                        sync.RWMutex
	refreshMu                 sync.Mutex
	lastUnknownRefresh        time.Time
}
type oidcClaims struct {
	jwt.RegisteredClaims
	Roles       []string `json:"roles,omitempty"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access,omitempty"`
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access,omitempty"`
}
type oidcDiscovery struct {
	Issuer  string `json:"issuer"`
	JWKSURL string `json:"jwks_uri"`
}
type oidcJWKS struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func newOIDCValidator(issuer, audience string) (*oidcValidator, error) {
	if issuer == "" || audience == "" {
		return nil, nil
	}
	issuer = strings.TrimRight(issuer, "/")
	var discovery oidcDiscovery
	if err := getJSON(issuer+"/.well-known/openid-configuration", &discovery); err != nil {
		return nil, err
	}
	if strings.TrimRight(discovery.Issuer, "/") != issuer || discovery.JWKSURL == "" {
		return nil, fmt.Errorf("OIDC discovery issuer or JWKS URI does not match configuration")
	}
	v := &oidcValidator{issuer: issuer, audience: audience, jwksURL: discovery.JWKSURL}
	if err := v.refresh(); err != nil {
		return nil, err
	}
	return v, nil
}
func (v *oidcValidator) refresh() error {
	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()
	return v.loadKeys()
}
func (v *oidcValidator) refreshForUnknownKey() error {
	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()
	if time.Since(v.lastUnknownRefresh) < 5*time.Second {
		return nil
	}
	v.lastUnknownRefresh = time.Now()
	return v.loadKeys()
}
func (v *oidcValidator) loadKeys() error {
	var set oidcJWKS
	if err := getJSON(v.jwksURL, &set); err != nil {
		return err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, key := range set.Keys {
		if key.Kty != "RSA" {
			continue
		}
		n, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			return err
		}
		e, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			return err
		}
		exponent := 0
		for _, b := range e {
			exponent = exponent*256 + int(b)
		}
		keys[key.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
	}
	v.mu.Lock()
	v.keys = keys
	v.mu.Unlock()
	return nil
}
func (v *oidcValidator) Validate(raw string) (*oidcClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &oidcClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		kid, _ := token.Header["kid"].(string)
		v.mu.RLock()
		key := v.keys[kid]
		v.mu.RUnlock()
		if key == nil {
			if err := v.refreshForUnknownKey(); err != nil {
				return nil, fmt.Errorf("refresh OIDC signing keys: %w", err)
			}
			v.mu.RLock()
			key = v.keys[kid]
			v.mu.RUnlock()
			if key == nil {
				return nil, fmt.Errorf("unknown signing key")
			}
		}
		return key, nil
	}, jwt.WithIssuer(v.issuer), jwt.WithAudience(v.audience), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*oidcClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
func getJSON(url string, target any) error {
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("OIDC endpoint returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}
