// Package vector provides vector types and operations for sblite's vector search feature.
// Vectors are stored as JSON arrays in TEXT columns and can be searched using
// cosine similarity, L2 distance, or dot product metrics.
package vector

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Vector represents a float64 slice for embedding vectors.
type Vector []float64

// vectorTypeRegex matches vector(N) format where N is the dimension.
var vectorTypeRegex = regexp.MustCompile(`^vector\((\d+)\)$`)

// ParseVector parses a JSON array string into a Vector.
// Accepts formats: "[0.1, 0.2, 0.3]" or "0.1, 0.2, 0.3"
func ParseVector(s string) (Vector, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty vector string")
	}

	// Handle JSON array format
	if strings.HasPrefix(s, "[") {
		var v Vector
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, fmt.Errorf("invalid vector JSON: %w", err)
		}
		return v, nil
	}

	// Handle comma-separated format (no brackets)
	parts := strings.Split(s, ",")
	v := make(Vector, len(parts))
	for i, part := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid vector element at position %d: %w", i, err)
		}
		v[i] = f
	}
	return v, nil
}

// Format converts a Vector to its JSON array representation.
func (v Vector) Format() string {
	if v == nil {
		return "null"
	}
	b, _ := json.Marshal([]float64(v))
	return string(b)
}

// Dimension returns the number of elements in the vector.
func (v Vector) Dimension() int {
	return len(v)
}

// ParseVectorType parses a vector type specification like "vector(1536)"
// and returns the dimension. Returns 0, false if the format is invalid.
func ParseVectorType(t string) (dim int, ok bool) {
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

// FormatVectorType creates a vector type string with the given dimension.
func FormatVectorType(dim int) string {
	return fmt.Sprintf("vector(%d)", dim)
}

// IsVectorType checks if a type string represents a vector type.
func IsVectorType(t string) bool {
	_, ok := ParseVectorType(t)
	return ok
}

// ValidateVectorValue validates a value intended for a vector column.
// Accepts []float64, []interface{} (from JSON), or string (JSON array).
// Returns the normalized Vector and any validation error.
func ValidateVectorValue(value any, expectedDim int) (Vector, error) {
	if value == nil {
		return nil, nil
	}

	var vec Vector

	switch v := value.(type) {
	case Vector:
		vec = v

	case []float64:
		vec = Vector(v)

	case []interface{}:
		vec = make(Vector, len(v))
		for i, elem := range v {
			switch f := elem.(type) {
			case float64:
				vec[i] = f
			case int:
				vec[i] = float64(f)
			case int64:
				vec[i] = float64(f)
			default:
				return nil, fmt.Errorf("vector element at position %d must be a number, got %T", i, elem)
			}
		}

	case string:
		var err error
		vec, err = ParseVector(v)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("vector must be an array of numbers or JSON string, got %T", value)
	}

	// Validate dimension if specified
	if expectedDim > 0 && len(vec) != expectedDim {
		return nil, fmt.Errorf("vector dimension mismatch: expected %d, got %d", expectedDim, len(vec))
	}

	// Validate all elements are valid numbers (not NaN or Inf)
	for i, f := range vec {
		if f != f { // NaN check
			return nil, fmt.Errorf("vector element at position %d is NaN", i)
		}
	}

	return vec, nil
}
