package storage

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// SignedURLUploadExpiry is the fixed expiry for upload signed URLs (2 hours)
	SignedURLUploadExpiry = 7200
)

// DownloadClaims represents the JWT claims for a download signed URL.
type DownloadClaims struct {
	URL  string `json:"url"`
	Type string `json:"type"`
	jwt.RegisteredClaims
}

// UploadClaims represents the JWT claims for an upload signed URL.
type UploadClaims struct {
	URL     string `json:"url"`
	Type    string `json:"type"`
	OwnerID string `json:"owner_id,omitempty"`
	Upsert  bool   `json:"upsert,omitempty"`
	jwt.RegisteredClaims
}

// GenerateDownloadToken creates a signed JWT for downloading a file.
func GenerateDownloadToken(bucket, path string, expiresIn int, secret string) (string, error) {
	now := time.Now()
	claims := DownloadClaims{
		URL:  bucket + "/" + path,
		Type: "storage-download",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expiresIn) * time.Second)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateDownloadToken validates a download signed URL token.
func ValidateDownloadToken(tokenString, secret string) (*DownloadClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &DownloadClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*DownloadClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.Type != "storage-download" {
		return nil, fmt.Errorf("invalid token type: expected storage-download")
	}

	return claims, nil
}

// GenerateUploadToken creates a signed JWT for uploading a file.
func GenerateUploadToken(bucket, path, ownerID string, upsert bool, secret string) (string, error) {
	now := time.Now()
	claims := UploadClaims{
		URL:     bucket + "/" + path,
		Type:    "storage-upload",
		OwnerID: ownerID,
		Upsert:  upsert,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(SignedURLUploadExpiry) * time.Second)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateUploadToken validates an upload signed URL token.
func ValidateUploadToken(tokenString, secret string) (*UploadClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UploadClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*UploadClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.Type != "storage-upload" {
		return nil, fmt.Errorf("invalid token type: expected storage-upload")
	}

	return claims, nil
}
