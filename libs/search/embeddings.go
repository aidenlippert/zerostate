// Package search provides semantic search capabilities for agent discovery.
package search

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strings"
)

// Vector represents a high-dimensional embedding vector
type Vector []float64

// Embedding generates vector embeddings from text using a simple hash-based approach.
// In production, this would use a pre-trained model like BERT or sentence-transformers.
type Embedding struct {
	dimensions int
}

// NewEmbedding creates a new embedding generator
func NewEmbedding(dimensions int) *Embedding {
	return &Embedding{
		dimensions: dimensions,
	}
}

// Encode converts text into a vector embedding
// This is a simplified implementation using feature hashing.
// Production would use transformer models or pre-trained embeddings.
func (e *Embedding) Encode(text string) Vector {
	text = strings.ToLower(strings.TrimSpace(text))
	words := strings.Fields(text)
	
	vec := make(Vector, e.dimensions)
	
	// Simple feature hashing with multiple hash functions
	for _, word := range words {
		// Use multiple hash seeds for better distribution
		for seed := 0; seed < 3; seed++ {
			hash := e.hashWord(word, seed)
			idx := int(hash % uint64(e.dimensions))
			
			// Add to vector with decay based on word position
			vec[idx] += 1.0 / float64(len(words))
		}
	}
	
	// Normalize to unit vector
	return e.normalize(vec)
}

// EncodeCapabilities creates an embedding from capability names and metadata
func (e *Embedding) EncodeCapabilities(capabilities []string, metadata map[string]string) Vector {
	// Combine capability names and metadata into text
	var parts []string
	parts = append(parts, capabilities...)
	
	for key, value := range metadata {
		parts = append(parts, key+":"+value)
	}
	
	text := strings.Join(parts, " ")
	return e.Encode(text)
}

// hashWord generates a hash for a word with a seed
func (e *Embedding) hashWord(word string, seed int) uint64 {
	h := sha256.New()
	h.Write([]byte(word))
	h.Write([]byte{byte(seed)})
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}

// normalize converts a vector to unit length
func (e *Embedding) normalize(vec Vector) Vector {
	var sumSquares float64
	for _, v := range vec {
		sumSquares += v * v
	}
	
	if sumSquares == 0 {
		return vec
	}
	
	magnitude := math.Sqrt(sumSquares)
	normalized := make(Vector, len(vec))
	for i, v := range vec {
		normalized[i] = v / magnitude
	}
	
	return normalized
}

// CosineSimilarity computes the cosine similarity between two vectors
// Returns a value between -1 and 1, where 1 means identical direction
func CosineSimilarity(a, b Vector) float64 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct float64
	for i := range a {
		dotProduct += a[i] * b[i]
	}
	
	return dotProduct // Both vectors are already normalized
}

// EuclideanDistance computes the Euclidean distance between two vectors
func EuclideanDistance(a, b Vector) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}
	
	var sumSquares float64
	for i := range a {
		diff := a[i] - b[i]
		sumSquares += diff * diff
	}
	
	return math.Sqrt(sumSquares)
}

// DotProduct computes the dot product of two vectors
func DotProduct(a, b Vector) float64 {
	if len(a) != len(b) {
		return 0
	}
	
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	
	return sum
}

// Add performs element-wise addition
func (v Vector) Add(other Vector) Vector {
	if len(v) != len(other) {
		return v
	}
	
	result := make(Vector, len(v))
	for i := range v {
		result[i] = v[i] + other[i]
	}
	
	return result
}

// Scale multiplies the vector by a scalar
func (v Vector) Scale(scalar float64) Vector {
	result := make(Vector, len(v))
	for i := range v {
		result[i] = v[i] * scalar
	}
	
	return result
}

// Magnitude returns the L2 norm (magnitude) of the vector
func (v Vector) Magnitude() float64 {
	var sum float64
	for _, val := range v {
		sum += val * val
	}
	return math.Sqrt(sum)
}
