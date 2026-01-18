// Package oauth provides OAuth 2.0 authentication support including
// PKCE (Proof Key for Code Exchange) for secure authorization flows.
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateCodeVerifier generates a cryptographically random code verifier
// for PKCE (RFC 7636). Returns a 43-character URL-safe string.
func GenerateCodeVerifier() (string, error) {
	// 32 bytes = 43 characters in base64url (without padding)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge computes the S256 code challenge from a verifier.
func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// GenerateState generates a cryptographically random state parameter
// for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
