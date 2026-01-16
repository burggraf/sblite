// internal/auth/apikey.go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// APIKeyType represents the type of API key
type APIKeyType string

const (
	APIKeyAnon        APIKeyType = "anon"
	APIKeyServiceRole APIKeyType = "service_role"
)

// GenerateAPIKey creates a JWT API key with role claim, no expiration
func (s *Service) GenerateAPIKey(keyType APIKeyType) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"role": string(keyType),
		"iss":  "sblite",
		"iat":  now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// ValidateAPIKey validates a JWT API key and returns the role
func (s *Service) ValidateAPIKey(tokenString string) (role string, err error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("invalid API key: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid API key claims")
	}

	role, ok = claims["role"].(string)
	if !ok {
		return "", fmt.Errorf("API key missing role claim")
	}

	// Validate role is one of the expected values
	if role != string(APIKeyAnon) && role != string(APIKeyServiceRole) {
		return "", fmt.Errorf("invalid API key role: %s", role)
	}

	return role, nil
}

// GetJWTSecret returns the JWT secret (used by CLI for key generation)
func (s *Service) GetJWTSecret() string {
	return s.jwtSecret
}
