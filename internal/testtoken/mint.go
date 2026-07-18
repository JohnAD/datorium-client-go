// Package testtoken mints development-only EdDSA client JWTs matching
// DatoriumDB fixture __auth.json + signing key material.
package testtoken

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// AuthFile is the subset of __auth.json needed to mint tokens.
type AuthFile struct {
	Auth struct {
		Issuer               string `json:"issuer"`
		Audience             string `json:"audience"`
		TokenLifetimeSeconds struct {
			Client  int `json:"client"`
			Machine int `json:"machine"`
		} `json:"tokenLifetimeSeconds"`
		Keys []struct {
			Kid       string `json:"kid"`
			Alg       string `json:"alg"`
			Status    string `json:"status"`
			PublicKey string `json:"publicKey"`
		} `json:"keys"`
	} `json:"auth"`
}

// MintClientToken signs a client JWT using a PKCS8 Ed25519 PEM private key
// and the issuer/audience/kid from authJSONPath (__auth.json).
func MintClientToken(authJSONPath, privateKeyPEMPath, subject string, lifetime time.Duration) (string, error) {
	raw, err := os.ReadFile(authJSONPath)
	if err != nil {
		return "", err
	}
	var auth AuthFile
	if err := json.Unmarshal(raw, &auth); err != nil {
		return "", err
	}
	priv, err := loadEd25519PrivateKeyPEM(privateKeyPEMPath)
	if err != nil {
		return "", err
	}
	kid := ""
	for _, k := range auth.Auth.Keys {
		if k.Status == "active" {
			kid = k.Kid
			break
		}
	}
	if kid == "" {
		return "", fmt.Errorf("testtoken: no active key in %s", authJSONPath)
	}
	if lifetime <= 0 {
		lifetime = time.Hour
		if auth.Auth.TokenLifetimeSeconds.Client > 0 {
			lifetime = time.Duration(auth.Auth.TokenLifetimeSeconds.Client) * time.Second
		}
	}
	now := time.Now()
	tok, err := jwt.NewBuilder().
		Issuer(auth.Auth.Issuer).
		Audience([]string{auth.Auth.Audience}).
		Subject(subject).
		IssuedAt(now).
		Expiration(now.Add(lifetime)).
		Claim("datoriumdb.kind", "client").
		Build()
	if err != nil {
		return "", err
	}
	key, err := jwk.Import(priv)
	if err != nil {
		return "", err
	}
	if err := key.Set(jwk.KeyIDKey, kid); err != nil {
		return "", err
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.EdDSA(), key))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

func loadEd25519PrivateKeyPEM(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("testtoken: no PEM block in %s", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("testtoken: not an Ed25519 private key")
	}
	return priv, nil
}
