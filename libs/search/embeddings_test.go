package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbedding(t *testing.T) {
	emb := NewEmbedding(128)
	
	// Test basic encoding
	vec1 := emb.Encode("text generation language model")
	assert.Len(t, vec1, 128, "Vector should have correct dimensions")
	
	// Check normalization (unit vector)
	mag := vec1.Magnitude()
	assert.InDelta(t, 1.0, mag, 0.01, "Vector should be normalized")
	
	// Similar text should have similar embeddings
	vec2 := emb.Encode("text generation language")
	similarity := CosineSimilarity(vec1, vec2)
	assert.Greater(t, similarity, 0.5, "Similar text should have high similarity")
	
	// Different text should have lower similarity
	vec3 := emb.Encode("image processing computer vision")
	similarity2 := CosineSimilarity(vec1, vec3)
	assert.Less(t, similarity2, similarity, "Different text should have lower similarity")
}

func TestCapabilityEmbedding(t *testing.T) {
	emb := NewEmbedding(128)
	
	caps1 := []string{"text-generation", "summarization"}
	meta1 := map[string]string{
		"model": "gpt-4",
		"max_tokens": "8192",
	}
	
	vec1 := emb.EncodeCapabilities(caps1, meta1)
	assert.Len(t, vec1, 128)
	
	// Similar capabilities
	caps2 := []string{"text-generation", "completion"}
	vec2 := emb.EncodeCapabilities(caps2, nil)
	
	similarity := CosineSimilarity(vec1, vec2)
	assert.Greater(t, similarity, 0.3, "Similar capabilities should have some similarity")
}

func TestVectorOperations(t *testing.T) {
	v1 := Vector{1.0, 2.0, 3.0}
	v2 := Vector{2.0, 3.0, 4.0}
	
	// Test Add
	v3 := v1.Add(v2)
	assert.Equal(t, Vector{3.0, 5.0, 7.0}, v3)
	
	// Test Scale
	v4 := v1.Scale(2.0)
	assert.Equal(t, Vector{2.0, 4.0, 6.0}, v4)
	
	// Test DotProduct
	dot := DotProduct(v1, v2)
	assert.Equal(t, 20.0, dot) // 1*2 + 2*3 + 3*4 = 20
	
	// Test EuclideanDistance
	dist := EuclideanDistance(v1, v2)
	expected := 1.7320508075688772 // sqrt((2-1)^2 + (3-2)^2 + (4-3)^2)
	assert.InDelta(t, expected, dist, 0.0001)
}

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors
	v1 := Vector{1.0, 0.0, 0.0}
	v2 := Vector{1.0, 0.0, 0.0}
	sim := CosineSimilarity(v1, v2)
	assert.InDelta(t, 1.0, sim, 0.001, "Identical vectors should have similarity 1.0")
	
	// Orthogonal vectors
	v3 := Vector{1.0, 0.0, 0.0}
	v4 := Vector{0.0, 1.0, 0.0}
	sim2 := CosineSimilarity(v3, v4)
	assert.InDelta(t, 0.0, sim2, 0.001, "Orthogonal vectors should have similarity 0.0")
}
