package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	require.NoError(t, err)

	// RFC 7636: verifier must be 43-128 characters
	assert.GreaterOrEqual(t, len(verifier), 43)
	assert.LessOrEqual(t, len(verifier), 128)

	// Should be URL-safe base64
	for _, c := range verifier {
		assert.True(t, isURLSafeBase64Char(c), "character %c should be URL-safe", c)
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := GenerateCodeChallenge(verifier)

	// Manually compute expected challenge
	hash := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(hash[:])

	assert.Equal(t, expected, challenge)
}

func TestGenerateState(t *testing.T) {
	state, err := GenerateState()
	require.NoError(t, err)

	// State should be at least 32 characters for security
	assert.GreaterOrEqual(t, len(state), 32)
}

func isURLSafeBase64Char(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_'
}
