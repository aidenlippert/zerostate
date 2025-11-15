package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthBlockchainResponse represents blockchain health status
type HealthBlockchainResponse struct {
	Enabled        bool                   `json:"enabled"`
	Connected      bool                   `json:"connected"`
	ChainName      string                 `json:"chain_name,omitempty"`
	ChainVersion   string                 `json:"chain_version,omitempty"`
	BlockHeight    uint64                 `json:"block_height,omitempty"`
	LastCheck      time.Time              `json:"last_check"`
	CircuitBreaker map[string]interface{} `json:"circuit_breaker,omitempty"`
	Metrics        map[string]interface{} `json:"metrics,omitempty"`
	Status         string                 `json:"status"` // "healthy", "degraded", "unhealthy"
}

// HandleHealthBlockchain handles GET /health/blockchain
func (h *Handlers) HandleHealthBlockchain(c *gin.Context) {
	response := HealthBlockchainResponse{
		LastCheck: time.Now(),
		Enabled:   false,
		Connected: false,
		Status:    "unhealthy",
	}

	// Check if blockchain is enabled
	if h.blockchain == nil || !h.blockchain.IsEnabled() {
		response.Status = "disabled"
		c.JSON(http.StatusOK, response)
		return
	}

	response.Enabled = true

	// Get chain info
	ctx := c.Request.Context()
	info, err := h.blockchain.GetChainInfo(ctx)
	if err != nil {
		response.Status = "unhealthy"
		response.Connected = false
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	response.Connected = true
	response.ChainName = info.Name
	response.ChainVersion = info.Version
	response.BlockHeight = info.BlockNumber

	// Get circuit breaker state
	response.CircuitBreaker = h.blockchain.GetCircuitBreakerStats()
	circuitState := h.blockchain.GetCircuitBreakerState()

	// Get metrics
	response.Metrics = h.blockchain.GetMetrics()

	// Determine overall status
	switch circuitState {
	case "open":
		response.Status = "unhealthy"
	case "half-open":
		response.Status = "degraded"
	case "closed":
		response.Status = "healthy"
	default:
		response.Status = "healthy"
	}

	statusCode := http.StatusOK
	if response.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if response.Status == "degraded" {
		statusCode = http.StatusOK // Still return 200 for degraded
	}

	c.JSON(statusCode, response)
}

// HandleHealthBlockchainMetrics handles GET /health/blockchain/metrics (detailed metrics)
func (h *Handlers) HandleHealthBlockchainMetrics(c *gin.Context) {
	if h.blockchain == nil || !h.blockchain.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "blockchain service not available",
		})
		return
	}

	metrics := h.blockchain.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// HandleHealthBlockchainCircuitBreaker handles GET /health/blockchain/circuit-breaker
func (h *Handlers) HandleHealthBlockchainCircuitBreaker(c *gin.Context) {
	if h.blockchain == nil || !h.blockchain.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "blockchain service not available",
		})
		return
	}

	stats := h.blockchain.GetCircuitBreakerStats()
	c.JSON(http.StatusOK, stats)
}
