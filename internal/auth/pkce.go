package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCEPair holds the verifier and challenge needed for the OAuth2 PKCE flow
type PKCEPair struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE creates a cryptographically random verifier and its SHA256 challenge
func GeneratePKCE() (PKCEPair, error) {
	// 32 random bytes → 43-character base64url string (RFC 7636 requires 43-128 chars)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCEPair{}, fmt.Errorf("generating verifier: %w", err)
	}

	verifier := base64.RawURLEncoding.EncodeToString(b)

	// Challenge = BASE64URL(SHA256(verifier))
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return PKCEPair{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}
