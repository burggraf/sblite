package vector

import (
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Vector
		want    float64
		wantErr bool
	}{
		{
			name: "identical vectors",
			a:    Vector{1, 0, 0},
			b:    Vector{1, 0, 0},
			want: 1.0,
		},
		{
			name: "opposite vectors",
			a:    Vector{1, 0, 0},
			b:    Vector{-1, 0, 0},
			want: -1.0,
		},
		{
			name: "orthogonal vectors",
			a:    Vector{1, 0, 0},
			b:    Vector{0, 1, 0},
			want: 0.0,
		},
		{
			name: "same direction different magnitude",
			a:    Vector{1, 0, 0},
			b:    Vector{5, 0, 0},
			want: 1.0,
		},
		{
			name: "45 degree angle",
			a:    Vector{1, 0},
			b:    Vector{1, 1},
			want: 1 / math.Sqrt(2), // cos(45°) ≈ 0.707
		},
		{
			name: "high-dimensional",
			a:    Vector{0.1, 0.2, 0.3, 0.4, 0.5},
			b:    Vector{0.5, 0.4, 0.3, 0.2, 0.1},
			want: 0.6363636363636364,
		},
		{
			name:    "dimension mismatch",
			a:       Vector{1, 0, 0},
			b:       Vector{1, 0},
			wantErr: true,
		},
		{
			name:    "empty vectors",
			a:       Vector{},
			b:       Vector{},
			wantErr: true,
		},
		{
			name: "zero vector",
			a:    Vector{0, 0, 0},
			b:    Vector{1, 0, 0},
			want: 0.0, // Zero vectors have no direction
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CosineSimilarity(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("CosineSimilarity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("CosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestL2Distance(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Vector
		want    float64
		wantErr bool
	}{
		{
			name: "identical vectors",
			a:    Vector{1, 0, 0},
			b:    Vector{1, 0, 0},
			want: 0.0,
		},
		{
			name: "unit distance",
			a:    Vector{0, 0, 0},
			b:    Vector{1, 0, 0},
			want: 1.0,
		},
		{
			name: "3-4-5 triangle",
			a:    Vector{0, 0},
			b:    Vector{3, 4},
			want: 5.0,
		},
		{
			name: "negative values",
			a:    Vector{-1, -1},
			b:    Vector{1, 1},
			want: 2 * math.Sqrt(2), // ~2.828
		},
		{
			name:    "dimension mismatch",
			a:       Vector{1, 0, 0},
			b:       Vector{1, 0},
			wantErr: true,
		},
		{
			name:    "empty vectors",
			a:       Vector{},
			b:       Vector{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := L2Distance(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("L2Distance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("L2Distance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Vector
		want    float64
		wantErr bool
	}{
		{
			name: "identical unit vectors",
			a:    Vector{1, 0, 0},
			b:    Vector{1, 0, 0},
			want: 1.0,
		},
		{
			name: "orthogonal",
			a:    Vector{1, 0, 0},
			b:    Vector{0, 1, 0},
			want: 0.0,
		},
		{
			name: "opposite",
			a:    Vector{1, 0, 0},
			b:    Vector{-1, 0, 0},
			want: -1.0,
		},
		{
			name: "general case",
			a:    Vector{1, 2, 3},
			b:    Vector{4, 5, 6},
			want: 32.0, // 1*4 + 2*5 + 3*6 = 32
		},
		{
			name:    "dimension mismatch",
			a:       Vector{1, 0, 0},
			b:       Vector{1, 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DotProduct(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("DotProduct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("DotProduct() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeSimilarity(t *testing.T) {
	a := Vector{1, 0, 0}
	b := Vector{0, 1, 0}

	// Cosine similarity for orthogonal vectors
	got, err := ComputeSimilarity(a, b, "cosine")
	if err != nil {
		t.Errorf("ComputeSimilarity(cosine) error = %v", err)
	}
	if math.Abs(got-0) > 1e-9 {
		t.Errorf("ComputeSimilarity(cosine) = %v, want 0", got)
	}

	// L2 returns negative distance
	got, err = ComputeSimilarity(a, b, "l2")
	if err != nil {
		t.Errorf("ComputeSimilarity(l2) error = %v", err)
	}
	if math.Abs(got-(-math.Sqrt(2))) > 1e-9 {
		t.Errorf("ComputeSimilarity(l2) = %v, want %v", got, -math.Sqrt(2))
	}

	// Dot product
	got, err = ComputeSimilarity(a, b, "dot")
	if err != nil {
		t.Errorf("ComputeSimilarity(dot) error = %v", err)
	}
	if math.Abs(got-0) > 1e-9 {
		t.Errorf("ComputeSimilarity(dot) = %v, want 0", got)
	}

	// Unknown metric
	_, err = ComputeSimilarity(a, b, "unknown")
	if err == nil {
		t.Error("ComputeSimilarity(unknown) should return error")
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		v    Vector
		want Vector
	}{
		{
			name: "unit vector unchanged",
			v:    Vector{1, 0, 0},
			want: Vector{1, 0, 0},
		},
		{
			name: "scale down",
			v:    Vector{3, 4, 0},
			want: Vector{0.6, 0.8, 0},
		},
		{
			name: "zero vector",
			v:    Vector{0, 0, 0},
			want: Vector{0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.v)
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 1e-9 {
					t.Errorf("Normalize()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMagnitude(t *testing.T) {
	tests := []struct {
		name string
		v    Vector
		want float64
	}{
		{
			name: "unit vector",
			v:    Vector{1, 0, 0},
			want: 1.0,
		},
		{
			name: "3-4-5 triangle",
			v:    Vector{3, 4},
			want: 5.0,
		},
		{
			name: "zero vector",
			v:    Vector{0, 0, 0},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Magnitude(tt.v)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Magnitude() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkCosineSimilarity(b *testing.B) {
	// 1536-dimensional vectors (OpenAI embedding size)
	a := make(Vector, 1536)
	v := make(Vector, 1536)
	for i := range a {
		a[i] = float64(i) / 1536
		v[i] = float64(1536-i) / 1536
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineSimilarity(a, v)
	}
}

func BenchmarkL2Distance(b *testing.B) {
	a := make(Vector, 1536)
	v := make(Vector, 1536)
	for i := range a {
		a[i] = float64(i) / 1536
		v[i] = float64(1536-i) / 1536
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		L2Distance(a, v)
	}
}
