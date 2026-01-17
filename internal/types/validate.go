package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Validator is a function that validates a value for a specific type.
type Validator func(value any) error

// validators maps PostgreSQL types to their validation functions.
var validators = map[PgType]Validator{
	TypeUUID:        validateUUID,
	TypeText:        validateText,
	TypeInteger:     validateInteger,
	TypeNumeric:     validateNumeric,
	TypeBoolean:     validateBoolean,
	TypeTimestamptz: validateTimestamptz,
	TypeJSONB:       validateJSONB,
	TypeBytea:       validateBytea,
}

// uuidRegex matches standard UUID format: 8-4-4-4-12 hex characters.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Validate checks if a value is valid for the given PostgreSQL type.
// Nil values are always valid (nullability is checked separately).
func Validate(pgType PgType, value any) error {
	// Nil values are always valid
	if value == nil {
		return nil
	}

	validator, ok := validators[pgType]
	if !ok {
		return fmt.Errorf("unknown type: %s", pgType)
	}

	return validator(value)
}

// validateUUID validates UUID format (8-4-4-4-12 hex pattern).
func validateUUID(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("uuid must be a string, got %T", value)
	}

	if !uuidRegex.MatchString(s) {
		return fmt.Errorf("invalid uuid format: %s", s)
	}

	return nil
}

// validateText validates text type (must be a string).
func validateText(value any) error {
	if _, ok := value.(string); !ok {
		return fmt.Errorf("text must be a string, got %T", value)
	}
	return nil
}

// validateInteger validates integer type.
// Accepts int, int32, int64, or float64 (whole numbers only).
// Must fit within int32 range.
func validateInteger(value any) error {
	var intValue int64

	switch v := value.(type) {
	case int:
		intValue = int64(v)
	case int32:
		intValue = int64(v)
	case int64:
		intValue = v
	case float64:
		if v != math.Trunc(v) {
			return fmt.Errorf("integer cannot have decimal part: %v", v)
		}
		intValue = int64(v)
	default:
		return fmt.Errorf("integer must be a numeric type, got %T", value)
	}

	// Check int32 range
	if intValue > math.MaxInt32 || intValue < math.MinInt32 {
		return fmt.Errorf("integer out of int32 range: %d", intValue)
	}

	return nil
}

// validateNumeric validates numeric/decimal type.
// Accepts numeric types or strings with valid decimal format.
func validateNumeric(value any) error {
	switch v := value.(type) {
	case int, int32, int64, float32, float64:
		return nil
	case string:
		if v == "" {
			return fmt.Errorf("numeric string cannot be empty")
		}
		// Try to parse as a number
		_, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid numeric format: %s", v)
		}
		return nil
	default:
		return fmt.Errorf("numeric must be a number or numeric string, got %T", value)
	}
}

// validateBoolean validates boolean type.
// Accepts bool or 0/1 as int/int64/float64.
func validateBoolean(value any) error {
	switch v := value.(type) {
	case bool:
		return nil
	case int:
		if v != 0 && v != 1 {
			return fmt.Errorf("boolean integer must be 0 or 1, got %d", v)
		}
		return nil
	case int64:
		if v != 0 && v != 1 {
			return fmt.Errorf("boolean integer must be 0 or 1, got %d", v)
		}
		return nil
	case float64:
		if v != 0 && v != 1 {
			return fmt.Errorf("boolean float must be 0 or 1, got %v", v)
		}
		return nil
	default:
		return fmt.Errorf("boolean must be bool or 0/1, got %T", value)
	}
}

// timestampFormats are the accepted ISO 8601 timestamp formats.
var timestampFormats = []string{
	time.RFC3339Nano,                // 2006-01-02T15:04:05.999999999Z07:00
	time.RFC3339,                    // 2006-01-02T15:04:05Z07:00
	"2006-01-02T15:04:05.999999999", // Without timezone
	"2006-01-02T15:04:05",           // Without timezone, no fraction
}

// validateTimestamptz validates timestamp with timezone.
// Accepts strings in ISO 8601 format.
func validateTimestamptz(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("timestamptz must be a string, got %T", value)
	}

	if s == "" {
		return fmt.Errorf("timestamptz cannot be empty")
	}

	for _, format := range timestampFormats {
		if _, err := time.Parse(format, s); err == nil {
			return nil
		}
	}

	return fmt.Errorf("invalid timestamptz format: %s", s)
}

// validateJSONB validates JSONB type.
// Accepts valid JSON strings (objects or arrays only, not primitives),
// or Go maps/slices.
func validateJSONB(value any) error {
	switch v := value.(type) {
	case string:
		if !json.Valid([]byte(v)) {
			return fmt.Errorf("invalid JSON: %s", v)
		}
		// Check if it's an object or array (not a primitive)
		trimmed := strings.TrimSpace(v)
		if len(trimmed) == 0 {
			return fmt.Errorf("empty JSON string")
		}
		if trimmed[0] != '{' && trimmed[0] != '[' {
			return fmt.Errorf("JSONB must be an object or array, not a primitive")
		}
		return nil
	case map[string]any, []any:
		return nil
	default:
		return fmt.Errorf("jsonb must be a JSON string, map, or slice, got %T", value)
	}
}

// validateBytea validates bytea (binary) type.
// Accepts []byte or valid base64 encoded strings (no whitespace allowed).
func validateBytea(value any) error {
	switch v := value.(type) {
	case []byte:
		return nil
	case string:
		if v == "" {
			return nil
		}
		// Reject base64 with whitespace (newlines, spaces, etc.)
		if strings.ContainsAny(v, " \t\n\r") {
			return fmt.Errorf("base64 string cannot contain whitespace")
		}
		// Validate base64 encoding
		_, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return fmt.Errorf("invalid base64 encoding: %v", err)
		}
		return nil
	default:
		return fmt.Errorf("bytea must be []byte or base64 string, got %T", value)
	}
}
