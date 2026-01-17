package types

import (
	"math"
	"testing"
)

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"uppercase uuid", "550E8400-E29B-41D4-A716-446655440000", false},
		{"invalid format", "not-a-uuid", true},
		{"too short", "550e8400-e29b-41d4", true},
		{"nil", nil, false},
		{"empty string", "", true},
		{"missing hyphens", "550e8400e29b41d4a716446655440000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeUUID, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeUUID, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateInteger(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"int", 42, false},
		{"int64", int64(42), false},
		{"float64 whole", float64(42), false},
		{"negative", -100, false},
		{"max int32", int32(math.MaxInt32), false},
		{"min int32", int32(math.MinInt32), false},
		{"overflow", int64(math.MaxInt32) + 1, true},
		{"underflow", int64(math.MinInt32) - 1, true},
		{"float with decimal", 42.5, true},
		{"string number", "42", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeInteger, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeInteger, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNumeric(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"decimal string", "123.456", false},
		{"negative", "-123.456", false},
		{"integer string", "42", false},
		{"leading zero", "0.123", false},
		{"float", 123.456, false},
		{"int", 42, false},
		{"double decimal", "12.34.56", true},
		{"letters", "abc", true},
		{"mixed", "12abc", true},
		{"nil", nil, false},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeNumeric, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeNumeric, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBoolean(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"int 0", 0, false},
		{"int 1", 1, false},
		{"float 0", float64(0), false},
		{"float 1", float64(1), false},
		{"int 2", 2, true},
		{"string true", "true", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeBoolean, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeBoolean, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimestamptz(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"ISO 8601 UTC", "2024-01-15T10:30:00Z", false},
		{"offset", "2024-01-15T10:30:00+05:00", false},
		{"no tz", "2024-01-15T10:30:00", false},
		{"date only", "2024-01-15", true},
		{"invalid format", "not-a-date", true},
		{"empty", "", true},
		{"nil", nil, false},
		{"with milliseconds", "2024-01-15T10:30:00.123Z", false},
		{"negative offset", "2024-01-15T10:30:00-08:00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeTimestamptz, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeTimestamptz, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateJSONB(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"object string", `{"key": "value"}`, false},
		{"array string", `[1, 2, 3]`, false},
		{"map", map[string]any{"key": "value"}, false},
		{"slice", []any{1, 2, 3}, false},
		{"invalid json", `{"key": }`, true},
		{"plain string", `"just a string"`, true},
		{"nil", nil, false},
		{"number json", "42", true},
		{"nested object", `{"outer": {"inner": "value"}}`, false},
		{"empty object", `{}`, false},
		{"empty array", `[]`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeJSONB, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeJSONB, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBytea(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid base64", "SGVsbG8gV29ybGQ=", false},
		{"padded", "YWJj", false},
		{"byte slice", []byte("hello"), false},
		{"invalid base64", "not!valid@base64", true},
		{"empty", "", false},
		{"nil", nil, false},
		{"base64 with newlines", "SGVs\nbG8=", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeBytea, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeBytea, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateText(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"string", "hello", false},
		{"empty", "", false},
		{"unicode", "hello \u4e16\u754c", false},
		{"nil", nil, false},
		{"number", 42, true},
		{"bool", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeText, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(TypeText, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUnknownType(t *testing.T) {
	err := Validate(PgType("unknown"), "value")
	if err == nil {
		t.Error("Validate with unknown type should return error")
	}
}
