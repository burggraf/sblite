package vector

import (
	"fmt"

	"github.com/markb/sblite/internal/types"
)

func init() {
	// Register vector validator with types package to avoid import cycles
	types.SetVectorValidator(Validate)
}

// Validate validates a value for a vector type with the given dimension.
// This is called by the types package when validating vector columns.
func Validate(value any, dim int) error {
	if value == nil {
		return nil
	}

	_, err := ValidateVectorValue(value, dim)
	return err
}

// ToStorageFormat converts a value to the storage format for vector columns.
// Vectors are stored as JSON arrays in TEXT columns.
func ToStorageFormat(value any, dim int) (string, error) {
	if value == nil {
		return "", nil
	}

	vec, err := ValidateVectorValue(value, dim)
	if err != nil {
		return "", err
	}

	return vec.Format(), nil
}

// FromStorageFormat converts a stored JSON array string back to a Vector.
func FromStorageFormat(stored string) (Vector, error) {
	if stored == "" || stored == "null" {
		return nil, nil
	}

	return ParseVector(stored)
}

// ValidateDimension checks if a dimension is valid for a vector type.
func ValidateDimension(dim int) error {
	if dim <= 0 {
		return fmt.Errorf("vector dimension must be positive, got %d", dim)
	}
	if dim > 65536 {
		return fmt.Errorf("vector dimension %d exceeds maximum of 65536", dim)
	}
	return nil
}
