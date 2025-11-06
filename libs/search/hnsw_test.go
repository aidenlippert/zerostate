package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHNSWBasic(t *testing.T) {
	hnsw := NewHNSWIndex(16, 200)
	
	// Add some vectors
	vectors := []Vector{
		{1.0, 0.0, 0.0},
		{0.9, 0.1, 0.0},
		{0.0, 1.0, 0.0},
		{0.0, 0.9, 0.1},
		{0.0, 0.0, 1.0},
	}
	
	payloads := []string{"A", "B", "C", "D", "E"}
	
	for i, vec := range vectors {
		id := hnsw.Add(vec, payloads[i])
		assert.Equal(t, i, id, "ID should match index")
	}
	
	assert.Equal(t, 5, hnsw.Size(), "Index should have 5 nodes")
	
	// Search for nearest neighbor to first vector
	query := Vector{1.0, 0.0, 0.0}
	results := hnsw.Search(query, 2)
	
	require.Len(t, results, 2, "Should return 2 results")
	assert.Equal(t, "A", results[0].Payload, "Closest should be A")
	assert.Equal(t, "B", results[1].Payload, "Second closest should be B")
	assert.Less(t, results[0].Distance, results[1].Distance, "Results should be sorted by distance")
}

func TestHNSWLargerIndex(t *testing.T) {
	hnsw := NewHNSWIndex(16, 200)
	emb := NewEmbedding(64)
	
	// Add various text embeddings
	texts := []string{
		"text generation language model",
		"text summarization natural language",
		"image classification computer vision",
		"image segmentation deep learning",
		"speech recognition audio processing",
		"speech synthesis text to speech",
		"machine translation multilingual",
	}
	
	for i, text := range texts {
		vec := emb.Encode(text)
		hnsw.Add(vec, i)
	}
	
	// Search for text-related
	queryVec := emb.Encode("text processing nlp")
	results := hnsw.Search(queryVec, 3)
	
	require.Len(t, results, 3, "Should return 3 results")
	
	// Results should be sorted by distance
	for i := 1; i < len(results); i++ {
		assert.LessOrEqual(t, results[i-1].Distance, results[i].Distance, "Results should be sorted by distance")
	}
	
	// Log results for debugging
	for _, result := range results {
		idx := result.Payload.(int)
		t.Logf("Result: %s (distance: %.4f)", texts[idx], result.Distance)
	}
}

func TestHNSWCosineDistance(t *testing.T) {
	hnsw := NewHNSWIndex(16, 200)
	
	// Use cosine distance
	hnsw.SetDistanceFunc(func(a, b Vector) float64 {
		return 1.0 - CosineSimilarity(a, b)
	})
	
	// Normalized vectors
	vectors := []Vector{
		{1.0, 0.0, 0.0},
		{0.707, 0.707, 0.0},
		{0.0, 1.0, 0.0},
	}
	
	for i, vec := range vectors {
		hnsw.Add(vec, i)
	}
	
	// Search for vector close to first
	query := Vector{0.95, 0.05, 0.0}
	results := hnsw.Search(query, 2)
	
	require.Len(t, results, 2)
	assert.Equal(t, 0, results[0].Payload, "Closest should be first vector")
}
