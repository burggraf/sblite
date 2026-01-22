package vector

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		dim     int
		wantErr bool
	}{
		{
			name:  "valid vector with correct dimension",
			value: []float64{0.1, 0.2, 0.3},
			dim:   3,
		},
		{
			name:  "nil is always valid",
			value: nil,
			dim:   3,
		},
		{
			name:    "wrong dimension",
			value:   []float64{0.1, 0.2},
			dim:     3,
			wantErr: true,
		},
		{
			name:  "valid JSON string",
			value: "[1.0, 2.0, 3.0]",
			dim:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.value, tt.dim)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToStorageFormat(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		dim     int
		want    string
		wantErr bool
	}{
		{
			name:  "float64 slice",
			value: []float64{0.1, 0.2, 0.3},
			dim:   3,
			want:  "[0.1,0.2,0.3]",
		},
		{
			name:  "interface slice",
			value: []interface{}{0.1, 0.2, 0.3},
			dim:   3,
			want:  "[0.1,0.2,0.3]",
		},
		{
			name:  "nil returns empty",
			value: nil,
			dim:   3,
			want:  "",
		},
		{
			name:    "dimension mismatch",
			value:   []float64{0.1, 0.2},
			dim:     3,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToStorageFormat(tt.value, tt.dim)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToStorageFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ToStorageFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromStorageFormat(t *testing.T) {
	tests := []struct {
		name    string
		stored  string
		want    Vector
		wantErr bool
	}{
		{
			name:   "valid JSON array",
			stored: "[0.1, 0.2, 0.3]",
			want:   Vector{0.1, 0.2, 0.3},
		},
		{
			name:   "empty string",
			stored: "",
			want:   nil,
		},
		{
			name:   "null string",
			stored: "null",
			want:   nil,
		},
		{
			name:    "invalid JSON",
			stored:  "not valid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromStorageFormat(tt.stored)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromStorageFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("FromStorageFormat() = %v, want %v", got, tt.want)
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("FromStorageFormat() element %d = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestValidateDimension(t *testing.T) {
	tests := []struct {
		dim     int
		wantErr bool
	}{
		{1536, false},
		{384, false},
		{1, false},
		{65536, false},
		{0, true},
		{-1, true},
		{65537, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := ValidateDimension(tt.dim)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDimension(%d) error = %v, wantErr %v", tt.dim, err, tt.wantErr)
			}
		})
	}
}
