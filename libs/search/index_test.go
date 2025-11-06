package search

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestIndexCard(t *testing.T) {
	idx := NewIndex(zap.NewNop())
	ctx := context.Background()
	
	cardData := `{
		"did": "did:zs:abc123",
		"name": "GPT Agent",
		"capabilities": [
			{
				"name": "text-generation",
				"type": "service",
				"metadata": {
					"model": "gpt-4",
					"max_tokens": "8192"
				}
			},
			{
				"name": "summarization",
				"type": "service"
			}
		]
	}`
	
	err := idx.IndexCard(ctx, []byte(cardData))
	require.NoError(t, err)
	
	stats := idx.Stats()
	assert.Equal(t, 1, stats["total_cards"])
	assert.Equal(t, 128, stats["vector_dim"])
}

func TestSearchByText(t *testing.T) {
	idx := NewIndex(zap.NewNop())
	ctx := context.Background()
	
	cards := []string{
		`{"did":"did:zs:1","capabilities":[{"name":"text-generation"},{"name":"summarization"}]}`,
		`{"did":"did:zs:2","capabilities":[{"name":"image-classification"},{"name":"object-detection"}]}`,
		`{"did":"did:zs:3","capabilities":[{"name":"speech-recognition"},{"name":"audio-transcription"}]}`,
	}
	
	for _, card := range cards {
		require.NoError(t, idx.IndexCard(ctx, []byte(card)))
	}
	
	// Search for text-related
	results, err := idx.Search(ctx, "text generation language model", 2)
	require.NoError(t, err)
	require.Len(t, results, 2)
	
	// First result should be text-related card
	assert.Equal(t, "did:zs:1", results[0].DID)
	
	for _, r := range results {
		t.Logf("Result: %s (capabilities: %v)", r.DID, r.Capabilities)
	}
}

func TestSearchByCapabilities(t *testing.T) {
	idx := NewIndex(zap.NewNop())
	ctx := context.Background()
	
	cards := []string{
		`{"did":"did:zs:1","capabilities":[{"name":"text-generation"},{"name":"summarization"}]}`,
		`{"did":"did:zs:2","capabilities":[{"name":"text-generation"},{"name":"translation"}]}`,
		`{"did":"did:zs:3","capabilities":[{"name":"image-classification"}]}`,
	}
	
	for _, card := range cards {
		require.NoError(t, idx.IndexCard(ctx, []byte(card)))
	}
	
	// Search for cards with text-generation capability
	results, err := idx.SearchByCapabilities(ctx, []string{"text-generation"}, 5)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should find 2 cards with text-generation")
	
	// Verify both have the capability
	for _, r := range results {
		assert.Contains(t, r.Capabilities, "text-generation")
	}
}

func TestSearchWithMetadata(t *testing.T) {
	idx := NewIndex(zap.NewNop())
	ctx := context.Background()
	
	cardData := `{
		"did": "did:zs:abc123",
		"name": "GPT-4 Agent",
		"version": "1.0.0",
		"capabilities": [
			{
				"name": "text-generation",
				"type": "service",
				"metadata": {
					"model": "gpt-4",
					"max_tokens": "8192",
					"supports_streaming": "true"
				}
			}
		],
		"metadata": {
			"provider": "openai",
			"cost_tier": "premium"
		}
	}`
	
	err := idx.IndexCard(ctx, []byte(cardData))
	require.NoError(t, err)
	
	// Search should consider metadata
	results, err := idx.Search(ctx, "openai gpt-4 premium text generation", 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	
	assert.Equal(t, "did:zs:abc123", results[0].DID)
}

func TestInvalidJSON(t *testing.T) {
	idx := NewIndex(zap.NewNop())
	ctx := context.Background()
	
	err := idx.IndexCard(ctx, []byte("invalid json{"))
	assert.Error(t, err, "Should error on invalid JSON")
}

func TestEmptyCapabilities(t *testing.T) {
	idx := NewIndex(zap.NewNop())
	ctx := context.Background()
	
	cardData := `{
		"did": "did:zs:empty",
		"capabilities": []
	}`
	
	err := idx.IndexCard(ctx, []byte(cardData))
	require.NoError(t, err)
	
	// Should still be retrievable
	card, ok := idx.GetCard("did:zs:empty")
	require.True(t, ok)
	assert.Equal(t, "did:zs:empty", card.DID)
}

func TestLargeIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large index test in short mode")
	}
	
	idx := NewIndex(zap.NewNop())
	ctx := context.Background()
	
	// Add 100 cards
	for i := 0; i < 100; i++ {
		card := map[string]interface{}{
			"did": fmt.Sprintf("did:zs:%d", i),
			"capabilities": []map[string]string{
				{"name": fmt.Sprintf("capability-%d", i%10)},
			},
		}
		cardJSON, _ := json.Marshal(card)
		require.NoError(t, idx.IndexCard(ctx, cardJSON))
	}
	
	stats := idx.Stats()
	assert.Equal(t, 100, stats["total_cards"])
	
	// Search should still be fast
	results, err := idx.Search(ctx, "capability", 10)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 10)
}
