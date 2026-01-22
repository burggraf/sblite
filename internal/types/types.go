// Package types provides PostgreSQL type definitions and validation for sblite.
package types

import (
	"regexp"
	"strconv"
	"strings"
)

// PgType represents a supported PostgreSQL type.
type PgType string

const (
	TypeUUID        PgType = "uuid"
	TypeText        PgType = "text"
	TypeInteger     PgType = "integer"
	TypeNumeric     PgType = "numeric"
	TypeBoolean     PgType = "boolean"
	TypeTimestamptz PgType = "timestamptz"
	TypeJSONB       PgType = "jsonb"
	TypeBytea       PgType = "bytea"
)

// ValidTypes is the set of all supported types.
var ValidTypes = map[PgType]bool{
	TypeUUID:        true,
	TypeText:        true,
	TypeInteger:     true,
	TypeNumeric:     true,
	TypeBoolean:     true,
	TypeTimestamptz: true,
	TypeJSONB:       true,
	TypeBytea:       true,
}

// IsValidType checks if a type string is a supported type.
// Supports both standard types and parameterized types like vector(1536).
func IsValidType(t string) bool {
	if ValidTypes[PgType(t)] {
		return true
	}
	// Check for vector type
	return IsVectorType(t)
}

// vectorTypeRegex matches vector(N) format where N is the dimension.
var vectorTypeRegex = regexp.MustCompile(`^vector\((\d+)\)$`)

// IsVectorType checks if a type string represents a vector type (e.g., "vector(1536)").
func IsVectorType(t string) bool {
	return vectorTypeRegex.MatchString(strings.ToLower(strings.TrimSpace(t)))
}

// GetVectorDimension extracts the dimension from a vector type string.
// Returns 0, false if the type is not a valid vector type.
func GetVectorDimension(t string) (int, bool) {
	t = strings.TrimSpace(strings.ToLower(t))
	matches := vectorTypeRegex.FindStringSubmatch(t)
	if len(matches) != 2 {
		return 0, false
	}

	dim, err := strconv.Atoi(matches[1])
	if err != nil || dim <= 0 {
		return 0, false
	}

	return dim, true
}
