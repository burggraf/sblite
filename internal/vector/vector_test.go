package vector

import (
	"testing"
)

func TestParseVector(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Vector
		wantErr bool
	}{
		{
			name:  "JSON array format",
			input: "[0.1, 0.2, 0.3]",
			want:  Vector{0.1, 0.2, 0.3},
		},
		{
			name:  "JSON array without spaces",
			input: "[0.1,0.2,0.3]",
			want:  Vector{0.1, 0.2, 0.3},
		},
		{
			name:  "comma separated format",
			input: "0.1, 0.2, 0.3",
			want:  Vector{0.1, 0.2, 0.3},
		},
		{
			name:  "negative values",
			input: "[-0.5, 0.0, 0.5]",
			want:  Vector{-0.5, 0.0, 0.5},
		},
		{
			name:  "single element",
			input: "[1.5]",
			want:  Vector{1.5},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   "[0.1, 0.2,]",
			wantErr: true,
		},
		{
			name:    "non-numeric values",
			input:   "[0.1, \"text\", 0.3]",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVector(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ParseVector() got %v, want %v", got, tt.want)
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("ParseVector() got[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestVectorFormat(t *testing.T) {
	tests := []struct {
		name string
		v    Vector
		want string
	}{
		{
			name: "three elements",
			v:    Vector{0.1, 0.2, 0.3},
			want: "[0.1,0.2,0.3]",
		},
		{
			name: "negative values",
			v:    Vector{-0.5, 0.0, 0.5},
			want: "[-0.5,0,0.5]",
		},
		{
			name: "nil vector",
			v:    nil,
			want: "null",
		},
		{
			name: "empty vector",
			v:    Vector{},
			want: "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Format()
			if got != tt.want {
				t.Errorf("Vector.Format() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseVectorType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantDim int
		wantOK  bool
	}{
		{
			name:    "vector(1536)",
			input:   "vector(1536)",
			wantDim: 1536,
			wantOK:  true,
		},
		{
			name:    "vector(384)",
			input:   "vector(384)",
			wantDim: 384,
			wantOK:  true,
		},
		{
			name:    "uppercase VECTOR(128)",
			input:   "VECTOR(128)",
			wantDim: 128,
			wantOK:  true,
		},
		{
			name:    "with spaces",
			input:   " vector(1536) ",
			wantDim: 1536,
			wantOK:  true,
		},
		{
			name:   "just vector",
			input:  "vector",
			wantOK: false,
		},
		{
			name:   "invalid dimension",
			input:  "vector(0)",
			wantOK: false,
		},
		{
			name:   "negative dimension",
			input:  "vector(-1)",
			wantOK: false,
		},
		{
			name:   "text type",
			input:  "text",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDim, gotOK := ParseVectorType(tt.input)
			if gotOK != tt.wantOK {
				t.Errorf("ParseVectorType() ok = %v, want %v", gotOK, tt.wantOK)
				return
			}
			if gotDim != tt.wantDim {
				t.Errorf("ParseVectorType() dim = %v, want %v", gotDim, tt.wantDim)
			}
		})
	}
}

func TestIsVectorType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"vector(1536)", true},
		{"vector(384)", true},
		{"VECTOR(128)", true},
		{"vector", false},
		{"text", false},
		{"uuid", false},
		{"integer", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsVectorType(tt.input)
			if got != tt.want {
				t.Errorf("IsVectorType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateVectorValue(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		expectedDim int
		wantErr     bool
	}{
		{
			name:        "valid []float64",
			value:       []float64{0.1, 0.2, 0.3},
			expectedDim: 3,
		},
		{
			name:        "valid []interface{}",
			value:       []interface{}{0.1, 0.2, 0.3},
			expectedDim: 3,
		},
		{
			name:        "valid string",
			value:       "[0.1, 0.2, 0.3]",
			expectedDim: 3,
		},
		{
			name:        "nil value",
			value:       nil,
			expectedDim: 3,
		},
		{
			name:        "dimension mismatch",
			value:       []float64{0.1, 0.2},
			expectedDim: 3,
			wantErr:     true,
		},
		{
			name:        "no dimension check",
			value:       []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			expectedDim: 0, // 0 means don't check
		},
		{
			name:        "invalid type",
			value:       "not a vector",
			expectedDim: 3,
			wantErr:     true,
		},
		{
			name:        "integer type",
			value:       42,
			expectedDim: 0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateVectorValue(tt.value, tt.expectedDim)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVectorValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatVectorType(t *testing.T) {
	if got := FormatVectorType(1536); got != "vector(1536)" {
		t.Errorf("FormatVectorType(1536) = %v, want vector(1536)", got)
	}
}
