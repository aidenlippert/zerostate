package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/metrics"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/aidenlippert/zerostate/libs/storage"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"github.com/aidenlippert/zerostate/libs/websocket"
	"github.com/gin-gonic/gin"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Handlers holds all API request handlers and their dependencies
type Handlers struct {
	// Core dependencies
	logger       *zap.Logger
	host         host.Host
	signer       *identity.Signer
	hnsw         *search.Index
	taskQueue    *orchestration.TaskQueue
	orchestrator *orchestration.Orchestrator
	db           *database.Database
	s3Storage    *storage.S3Storage
	wsHub        *websocket.Hub

	// WASM Execution components (Sprint 9)
	wasmRunner   *execution.WASMRunner
	resultStore  *execution.PostgresResultStore
	binaryStore  execution.BinaryStore
	execHandlers *ExecutionHandlers

	// Blockchain integration (Sprint 2)
	blockchain *substrate.BlockchainService

	// Monitoring and health (Sprint 6 Phase 3)
	metricsHandler        *MetricsHandler
	enhancedHealthHandler *EnhancedHealthHandler

	// Services (to be added)
	// userManager    *auth.UserManager
	// paymentService *payment.Service

	ctx     context.Context
	p2pHost host.Host // Add explicit p2pHost field for health checks

	// Runtime registry & metrics
	runtimeRegistry *orchestration.RuntimeRegistry
	promMetrics     *metrics.PrometheusMetrics
	promRegistry    *prometheus.Registry
}

// NewHandlers creates a new Handlers instance
func NewHandlers(
	ctx context.Context,
	logger *zap.Logger,
	host host.Host,
	signer *identity.Signer,
	hnsw *search.Index,
	taskQueue *orchestration.TaskQueue,
	orchestrator *orchestration.Orchestrator,
	db *database.Database,
	s3Storage *storage.S3Storage,
	wsHub *websocket.Hub,
	wasmRunner *execution.WASMRunner,
	resultStore *execution.PostgresResultStore,
	binaryStore execution.BinaryStore,
	blockchain *substrate.BlockchainService,
	runtimeRegistry *orchestration.RuntimeRegistry,
	promMetrics *metrics.PrometheusMetrics,
	promRegistry *prometheus.Registry,
) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create execution handlers
	execHandlers := NewExecutionHandlers(logger, db, wasmRunner, resultStore, binaryStore)

	// Create handlers struct first
	handlers := &Handlers{
		logger:          logger,
		host:            host,
		signer:          signer,
		hnsw:            hnsw,
		taskQueue:       taskQueue,
		orchestrator:    orchestrator,
		db:              db,
		s3Storage:       s3Storage,
		wsHub:           wsHub,
		wasmRunner:      wasmRunner,
		resultStore:     resultStore,
		binaryStore:     binaryStore,
		execHandlers:    execHandlers,
		blockchain:      blockchain,
		ctx:             ctx,
		p2pHost:         host,
		runtimeRegistry: runtimeRegistry,
		promMetrics:     promMetrics,
		promRegistry:    promRegistry,
	}

	// Create monitoring handlers (Sprint 6 Phase 3)
	metricsHandler := NewMetricsHandler(handlers, logger, promMetrics, promRegistry)
	enhancedHealthHandler := NewEnhancedHealthHandler(handlers, metricsHandler, logger)

	// Set the handlers
	handlers.metricsHandler = metricsHandler
	handlers.enhancedHealthHandler = enhancedHealthHandler

	return handlers
}

// Context returns the handlers' context
func (h *Handlers) Context() context.Context {
	if h.ctx == nil {
		return context.Background()
	}
	return h.ctx
}

// Logger returns the handlers' logger
func (h *Handlers) Logger() *zap.Logger {
	return h.logger
}

// Execution handler delegation methods (Sprint 9)

// ExecuteTaskDirect delegates to execution handlers
func (h *Handlers) ExecuteTaskDirect(c *gin.Context) {
	h.execHandlers.ExecuteTaskDirect(c)
}

// ListTaskResults delegates to execution handlers
func (h *Handlers) ListTaskResults(c *gin.Context) {
	h.execHandlers.ListTaskResults(c)
}

// Monitoring and health handler delegation methods (Sprint 6 Phase 3)

// HandleMetrics serves Prometheus metrics
func (h *Handlers) HandleMetrics() gin.HandlerFunc {
	if h.metricsHandler != nil {
		return h.metricsHandler.HandleMetrics()
	}
	return gin.HandlerFunc(func(c *gin.Context) {
		c.JSON(500, gin.H{"error": "metrics handler not initialized"})
	})
}

// HandleMetricsSummary provides JSON metrics summary
func (h *Handlers) HandleMetricsSummary(c *gin.Context) {
	if h.metricsHandler != nil {
		h.metricsHandler.HandleMetricsSummary(c)
	} else {
		c.JSON(500, gin.H{"error": "metrics handler not initialized"})
	}
}

// HandleHealthMetrics provides detailed health metrics
func (h *Handlers) HandleHealthMetrics(c *gin.Context) {
	if h.metricsHandler != nil {
		h.metricsHandler.HandleHealthMetrics(c)
	} else {
		c.JSON(500, gin.H{"error": "metrics handler not initialized"})
	}
}

// HandleEnhancedHealth provides comprehensive health check
func (h *Handlers) HandleEnhancedHealth(c *gin.Context) {
	if h.enhancedHealthHandler != nil {
		h.enhancedHealthHandler.HandleDetailedHealth(c)
	} else {
		c.JSON(500, gin.H{"error": "health handler not initialized"})
	}
}

// HandleBasicHealth provides basic health check
func (h *Handlers) HandleBasicHealth(c *gin.Context) {
	if h.enhancedHealthHandler != nil {
		h.enhancedHealthHandler.HandleHealth(c)
	} else {
		c.JSON(200, gin.H{"status": "ok", "message": "basic health check"})
	}
}

// HandleReadiness provides readiness check for Kubernetes
func (h *Handlers) HandleReadiness(c *gin.Context) {
	if h.enhancedHealthHandler != nil {
		h.enhancedHealthHandler.HandleReadiness(c)
	} else {
		c.JSON(200, gin.H{"ready": true, "message": "basic readiness check"})
	}
}

// GetMetricsHandler returns the metrics handler for direct access
func (h *Handlers) GetMetricsHandler() *MetricsHandler {
	return h.metricsHandler
}

// ListRuntimeRegistry returns all discovered runtimes
func (h *Handlers) ListRuntimeRegistry(c *gin.Context) {
	if h.runtimeRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "runtime registry not enabled",
			"message": "P2P ARI executor is disabled on this node",
		})
		return
	}

	runtimes := h.runtimeRegistry.GetAllRuntimes()
	c.JSON(http.StatusOK, gin.H{
		"count":    len(runtimes),
		"runtimes": runtimes,
	})
}

// GetRuntimeRegistryEntry returns a single runtime by DID
func (h *Handlers) GetRuntimeRegistryEntry(c *gin.Context) {
	if h.runtimeRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "runtime registry not enabled",
		})
		return
	}

	did := c.Param("did")
	if strings.TrimSpace(did) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runtime DID is required"})
		return
	}

	runtime := h.runtimeRegistry.GetRuntime(did)
	if runtime == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "runtime not found",
			"runtime": did,
		})
		return
	}

	c.JSON(http.StatusOK, runtime)
}

// GetRuntimeRegistryHealth returns aggregate registry statistics
func (h *Handlers) GetRuntimeRegistryHealth(c *gin.Context) {
	if h.runtimeRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "disabled",
			"error":  "runtime registry not enabled",
		})
		return
	}

	stats := h.runtimeRegistry.GetStats()
	status := "healthy"
	if stats.Total == 0 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"stats":  stats,
	})
}
