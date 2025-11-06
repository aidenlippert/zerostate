package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	searchLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zerostate_search_latency_seconds",
			Help:    "Latency of semantic search queries",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"operation"},
	)

	indexSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zerostate_search_index_size",
			Help: "Number of entries in the search index",
		},
	)
)

// AgentCard represents a minimal agent card for search
type AgentCard struct {
	DID          string                 `json:"did"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]string      `json:"metadata"`
	Vector       Vector                 `json:"vector,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	RawCard      map[string]interface{} `json:"raw_card,omitempty"`
}

// Index provides semantic search over agent cards
type Index struct {
	mu sync.RWMutex
	
	hnsw      *HNSWIndex
	embedding *Embedding
	cards     map[string]*AgentCard // DID -> card
	logger    *zap.Logger
	
	// Configuration
	vectorDim int
	hnswM     int
	hnswEf    int
}

// NewIndex creates a new semantic search index
func NewIndex(logger *zap.Logger) *Index {
	vectorDim := 128 // Embedding dimensions
	hnswM := 16      // Number of connections per layer
	hnswEf := 200    // Construction parameter
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	hnsw := NewHNSWIndex(hnswM, hnswEf)
	
	// Use cosine similarity for semantic search
	hnsw.SetDistanceFunc(func(a, b Vector) float64 {
		// Convert cosine similarity to distance (1 - similarity)
		return 1.0 - CosineSimilarity(a, b)
	})
	
	return &Index{
		hnsw:      hnsw,
		embedding: NewEmbedding(vectorDim),
		cards:     make(map[string]*AgentCard),
		logger:    logger,
		vectorDim: vectorDim,
		hnswM:     hnswM,
		hnswEf:    hnswEf,
	}
}

// IndexCard adds or updates an agent card in the search index
func (idx *Index) IndexCard(ctx context.Context, cardJSON []byte) error {
	start := time.Now()
	defer func() {
		searchLatency.WithLabelValues("index").Observe(time.Since(start).Seconds())
	}()
	
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	// Parse card
	var rawCard map[string]interface{}
	if err := json.Unmarshal(cardJSON, &rawCard); err != nil {
		return fmt.Errorf("failed to parse card: %w", err)
	}
	
	did, _ := rawCard["did"].(string)
	if did == "" {
		return fmt.Errorf("card missing DID")
	}
	
	// Extract capabilities
	capabilities := make([]string, 0)
	metadata := make(map[string]string)
	
	if caps, ok := rawCard["capabilities"].([]interface{}); ok {
		for _, c := range caps {
			if capMap, ok := c.(map[string]interface{}); ok {
				if name, ok := capMap["name"].(string); ok {
					capabilities = append(capabilities, name)
					
					// Extract metadata
					if meta, ok := capMap["metadata"].(map[string]interface{}); ok {
						for k, v := range meta {
							if str, ok := v.(string); ok {
								metadata[k] = str
							} else {
								metadata[k] = fmt.Sprintf("%v", v)
							}
						}
					}
				}
			}
		}
	}
	
	// Generate embedding
	vector := idx.embedding.EncodeCapabilities(capabilities, metadata)
	
	card := &AgentCard{
		DID:          did,
		Capabilities: capabilities,
		Metadata:     metadata,
		Vector:       vector,
		Timestamp:    time.Now(),
		RawCard:      rawCard,
	}
	
	// Add to HNSW index
	idx.hnsw.Add(vector, card)
	
	// Store in map
	idx.cards[did] = card
	
	indexSize.Set(float64(len(idx.cards)))
	
	idx.logger.Debug("indexed agent card",
		zap.String("did", did),
		zap.Strings("capabilities", capabilities),
		zap.Int("metadata_count", len(metadata)),
	)
	
	return nil
}

// Search finds agents matching a query string
func (idx *Index) Search(ctx context.Context, query string, limit int) ([]*AgentCard, error) {
	start := time.Now()
	defer func() {
		searchLatency.WithLabelValues("search").Observe(time.Since(start).Seconds())
	}()
	
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	if idx.hnsw.Size() == 0 {
		return nil, nil
	}
	
	// Generate query embedding
	queryVec := idx.embedding.Encode(query)
	
	// Search HNSW index
	results := idx.hnsw.Search(queryVec, limit)
	
	// Convert to agent cards
	cards := make([]*AgentCard, len(results))
	for i, result := range results {
		if card, ok := result.Payload.(*AgentCard); ok {
			cards[i] = card
			
			idx.logger.Debug("search result",
				zap.String("did", card.DID),
				zap.Float64("similarity", 1.0-result.Distance),
				zap.Strings("capabilities", card.Capabilities),
			)
		}
	}
	
	return cards, nil
}

// SearchByCapabilities finds agents with specific capabilities
func (idx *Index) SearchByCapabilities(ctx context.Context, capabilities []string, limit int) ([]*AgentCard, error) {
	start := time.Now()
	defer func() {
		searchLatency.WithLabelValues("search_capabilities").Observe(time.Since(start).Seconds())
	}()
	
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	if idx.hnsw.Size() == 0 {
		return nil, nil
	}
	
	// Generate query embedding from capabilities
	queryVec := idx.embedding.EncodeCapabilities(capabilities, nil)
	
	// Search HNSW index
	results := idx.hnsw.Search(queryVec, limit)
	
	// Convert to agent cards
	cards := make([]*AgentCard, 0, len(results))
	for _, result := range results {
		if card, ok := result.Payload.(*AgentCard); ok {
			// Filter: card must have at least one of the requested capabilities
			hasMatch := false
			for _, reqCap := range capabilities {
				for _, cardCap := range card.Capabilities {
					if cardCap == reqCap {
						hasMatch = true
						break
					}
				}
				if hasMatch {
					break
				}
			}
			
			if hasMatch {
				cards = append(cards, card)
			}
		}
	}
	
	return cards, nil
}

// GetCard retrieves a card by DID
func (idx *Index) GetCard(did string) (*AgentCard, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	card, ok := idx.cards[did]
	return card, ok
}

// RemoveCard removes a card from the index
func (idx *Index) RemoveCard(did string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	delete(idx.cards, did)
	indexSize.Set(float64(len(idx.cards)))
	
	// Note: HNSW doesn't support deletion, would need rebuild
	// For production, implement periodic index rebuilding
}

// Size returns the number of indexed cards
func (idx *Index) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.cards)
}

// ListAll returns all indexed cards (for debugging/admin)
func (idx *Index) ListAll() []*AgentCard {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	cards := make([]*AgentCard, 0, len(idx.cards))
	for _, card := range idx.cards {
		cards = append(cards, card)
	}
	
	return cards
}

// Stats returns index statistics
func (idx *Index) Stats() map[string]interface{} {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	return map[string]interface{}{
		"total_cards":     len(idx.cards),
		"hnsw_nodes":      idx.hnsw.Size(),
		"vector_dim":      idx.vectorDim,
		"hnsw_m":          idx.hnswM,
		"hnsw_ef":         idx.hnswEf,
	}
}
