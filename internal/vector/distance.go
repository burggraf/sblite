package vector

import (
	"fmt"
	"math"
)

// SupportedMetrics lists the available distance metrics.
var SupportedMetrics = []string{"cosine", "l2", "dot"}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical direction,
// 0 means orthogonal, and -1 means opposite direction.
// For normalized vectors (unit length), this equals the dot product.
func CosineSimilarity(a, b Vector) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return 0, fmt.Errorf("cannot compute similarity of empty vectors")
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0 || normB == 0 {
		return 0, nil // Zero vectors have no direction
	}

	similarity := dotProduct / (normA * normB)
	// Clamp to [-1, 1] to handle floating point errors
	if similarity > 1 {
		similarity = 1
	} else if similarity < -1 {
		similarity = -1
	}

	return similarity, nil
}

// L2Distance computes the Euclidean (L2) distance between two vectors.
// Returns a non-negative value where 0 means identical vectors.
// Smaller values indicate more similar vectors.
func L2Distance(a, b Vector) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return 0, fmt.Errorf("cannot compute distance of empty vectors")
	}

	var sum float64
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return math.Sqrt(sum), nil
}

// DotProduct computes the inner product of two vectors.
// For normalized vectors, this equals cosine similarity.
// Higher values indicate more similar vectors.
func DotProduct(a, b Vector) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return 0, fmt.Errorf("cannot compute dot product of empty vectors")
	}

	var result float64
	for i := range a {
		result += a[i] * b[i]
	}

	return result, nil
}

// ComputeSimilarity computes the similarity/distance between two vectors
// using the specified metric. For "cosine", returns similarity (higher = more similar).
// For "l2", returns negative distance (higher = more similar, i.e., closer).
// For "dot", returns the dot product (higher = more similar for normalized vectors).
func ComputeSimilarity(a, b Vector, metric string) (float64, error) {
	switch metric {
	case "cosine":
		return CosineSimilarity(a, b)
	case "l2":
		// Return negative distance so higher = more similar
		dist, err := L2Distance(a, b)
		if err != nil {
			return 0, err
		}
		return -dist, nil
	case "dot":
		return DotProduct(a, b)
	default:
		return 0, fmt.Errorf("unknown distance metric: %s (supported: cosine, l2, dot)", metric)
	}
}

// Normalize returns a unit vector (length 1) in the same direction.
// Returns a zero vector if the input has zero magnitude.
func Normalize(v Vector) Vector {
	var magnitude float64
	for _, x := range v {
		magnitude += x * x
	}
	magnitude = math.Sqrt(magnitude)

	if magnitude == 0 {
		return make(Vector, len(v))
	}

	result := make(Vector, len(v))
	for i, x := range v {
		result[i] = x / magnitude
	}
	return result
}

// Magnitude returns the L2 norm (length) of a vector.
func Magnitude(v Vector) float64 {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}
