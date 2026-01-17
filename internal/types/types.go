// Package types provides PostgreSQL type definitions and validation for sblite.
package types

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
func IsValidType(t string) bool {
	return ValidTypes[PgType(t)]
}
