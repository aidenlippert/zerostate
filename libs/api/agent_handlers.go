package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// RegisterAgent handles agent registration with existing identity/search modules
func (h *Handlers) RegisterAgent(c *gin.Context) {
	ctx := c.Request.Context()
	_ = ctx // Used for tracing when enabled
	logger := h.logger.With(zap.String("handler", "RegisterAgent"))

	// Tracing spans are optional (requires tracer to be passed in handlers)
	var span trace.Span
	// For now, spans are disabled until tracer is properly initialized
	_ = span // Unused for now

	logger.Info("agent registration request received",
		zap.String("client_ip", c.ClientIP()),
	)

	// Parse multipart form
	if err := c.Request.ParseMultipartForm(MaxWASMSize); err != nil {
		logger.Error("failed to parse multipart form", zap.Error(err))
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid multipart form")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "failed to parse multipart form",
		})
		return
	}

	// Get WASM binary file
	file, header, err := c.Request.FormFile("wasm_binary")
	if err != nil {
		logger.Error("failed to get WASM binary", zap.Error(err))
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "missing WASM binary")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "wasm_binary file is required",
		})
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > MaxWASMSize {
		logger.Error("WASM binary too large",
			zap.Int64("size", header.Size),
			zap.Int64("max_size", MaxWASMSize),
		)
		if span != nil {
			span.SetStatus(codes.Error, "WASM binary too large")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": fmt.Sprintf("WASM binary exceeds maximum size of %d bytes", MaxWASMSize),
		})
		return
	}

	if header.Size < MinWASMSize {
		logger.Error("WASM binary too small",
			zap.Int64("size", header.Size),
		)
		if span != nil {
			span.SetStatus(codes.Error, "WASM binary too small")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "WASM binary is suspiciously small",
		})
		return
	}

	// Read WASM binary into memory
	wasmData := make([]byte, header.Size)
	if _, err := io.ReadFull(file, wasmData); err != nil {
		logger.Error("failed to read WASM binary", zap.Error(err))
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read WASM binary")
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to read WASM binary",
		})
		return
	}

	// Validate WASM format (check magic bytes)
	if !isValidWASM(wasmData) {
		logger.Error("invalid WASM format")
		if span != nil {
			span.SetStatus(codes.Error, "invalid WASM format")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "file is not a valid WebAssembly binary",
		})
		return
	}

	// Calculate WASM hash
	wasmHash := calculateWASMHash(wasmData)
	logger.Info("WASM binary validated",
		zap.String("hash", wasmHash),
		zap.Int64("size", header.Size),
	)

	// Parse JSON request data
	var req RegisterAgentRequest
	if err := c.ShouldBind(&req); err != nil {
		logger.Error("failed to parse request", zap.Error(err))
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid request data")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Validate capabilities
	if len(req.Capabilities) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "at least one capability is required",
		})
		return
	}

	if len(req.Capabilities) > MaxCapabilities {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": fmt.Sprintf("maximum %d capabilities allowed", MaxCapabilities),
		})
		return
	}

	// Set default resources if not provided
	if req.Resources == nil {
		req.Resources = &AgentResources{
			CPULimit:    "500m",
			MemoryLimit: "128Mi",
			Timeout:     30,
		}
	}

	// Generate Agent Card using identity package
	agentCard := &identity.AgentCard{
		DID: h.signer.DID(),
		Keys: &identity.Keys{
			Signing: h.signer.PublicKeyBase58(),
		},
		Endpoints: &identity.Endpoints{
			Libp2p: []string{h.host.ID().String()},
		},
		Capabilities: convertToCapabilities(req.Capabilities, req.Pricing),
		Proof: &identity.Proof{
			Type:    "Ed25519Signature2020",
			Created: time.Now().Format(time.RFC3339),
		},
	}

	// Sign the agent card
	if err := h.signer.SignCard(agentCard); err != nil {
		logger.Error("failed to sign agent card", zap.Error(err))
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to sign agent card")
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to sign agent card",
		})
		return
	}

	// Update HNSW index with agent capabilities
	indexUpdated := false
	if h.hnsw != nil {
		// Create embedding for capabilities
		embeddingGen := search.NewEmbedding(128)
		vector := embeddingGen.EncodeCapabilities(req.Capabilities, nil)

		// Add to HNSW index
		idx := h.hnsw.Add(vector, agentCard)
		if idx >= 0 {
			indexUpdated = true
			logger.Info("HNSW index updated",
				zap.String("agent_id", h.signer.DID()),
				zap.Int("index_id", idx),
			)
		} else {
			logger.Error("failed to update HNSW index")
		}
	}

	// TODO: Store WASM binary (IPFS, S3, or local storage)
	// For now, we just hash it for verification

	// Add span attributes
	if span != nil {
		span.SetAttributes(
			attribute.String("agent_id", h.signer.DID()),
			attribute.String("agent_name", req.Name),
			attribute.String("wasm_hash", wasmHash),
			attribute.Int64("wasm_size", header.Size),
			attribute.Bool("card_published", true), // Card is signed
			attribute.Bool("index_updated", indexUpdated),
		)
		span.SetStatus(codes.Ok, "agent registered successfully")
	}

	// Return success response
	response := RegisterAgentResponse{
		AgentID:       h.signer.DID(),
		Name:          req.Name,
		Status:        "registered",
		WASMHash:      wasmHash,
		CardPublished: true, // Card is signed
		IndexUpdated:  indexUpdated,
		Timestamp:     time.Now(),
	}

	logger.Info("agent registered successfully",
		zap.String("agent_id", h.signer.DID()),
		zap.String("name", req.Name),
		zap.Bool("index_updated", indexUpdated),
	)

	c.JSON(http.StatusCreated, response)
}

// Helper functions

// convertToCapabilities converts capability names to identity.Capability structs
func convertToCapabilities(names []string, pricing *AgentPricing) []identity.Capability {
	capabilities := make([]identity.Capability, len(names))
	for i, name := range names {
		cap := identity.Capability{
			Name:    name,
			Version: "1.0",
		}

		if pricing != nil {
			cap.Cost = &identity.Cost{
				Unit:  pricing.Currency,
				Price: pricing.PerExecution,
			}
		}

		capabilities[i] = cap
	}
	return capabilities
}

// GetAgent retrieves an agent by ID
func (h *Handlers) GetAgent(c *gin.Context) {
	agentID := c.Param("id")
	logger := h.logger.With(zap.String("handler", "GetAgent"), zap.String("agent_id", agentID))

	// Mock data for now - will be replaced with database query
	agent := gin.H{
		"id":          agentID,
		"name":        "DataWeaver",
		"description": "Advanced data analysis agent for ETL processing and database management",
		"capabilities": []string{"data_analysis", "etl", "database"},
		"status":      "active",
		"price":       0.02,
		"tasks_completed": 1200000,
		"rating":      4.9,
		"created_at":  time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
		"updated_at":  time.Now().Add(-1 * 24 * time.Hour).Format(time.RFC3339),
	}

	logger.Info("retrieved agent")
	c.JSON(http.StatusOK, agent)
}

// ListAgents lists all agents with pagination
func (h *Handlers) ListAgents(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "ListAgents"))

	// Mock data for now - will be replaced with database query
	agents := []gin.H{
		{
			"id":          "agent_001",
			"name":        "DataWeaver",
			"description": "Advanced data analysis agent for ETL processing and database management",
			"capabilities": []string{"data_analysis", "etl", "database"},
			"status":      "active",
			"price":       0.02,
			"tasks_completed": 1200000,
			"rating":      4.9,
			"created_at":  time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":          "agent_002",
			"name":        "Synth-Net",
			"description": "API integration and data synchronization specialist",
			"capabilities": []string{"api", "sync", "integration"},
			"status":      "active",
			"price":       0.05,
			"tasks_completed": 890000,
			"rating":      4.8,
			"created_at":  time.Now().Add(-20 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":          "agent_003",
			"name":        "Code-Gen X",
			"description": "Automated code generation and refactoring assistant",
			"capabilities": []string{"code_gen", "refactor", "testing"},
			"status":      "active",
			"price":       25.00,
			"tasks_completed": 450000,
			"rating":      4.7,
			"created_at":  time.Now().Add(-15 * 24 * time.Hour).Format(time.RFC3339),
		},
	}

	logger.Info("listing agents", zap.Int("count", len(agents)))

	c.JSON(http.StatusOK, gin.H{
		"agents":      agents,
		"total":       len(agents),
		"page":        1,
		"total_pages": 1,
	})
}

// UpdateAgent updates an existing agent
func (h *Handlers) UpdateAgent(c *gin.Context) {
	agentID := c.Param("id")

	// TODO: Update agent
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "UpdateAgent endpoint not yet implemented",
		"agent_id": agentID,
	})
}

// DeleteAgent deletes an agent
func (h *Handlers) DeleteAgent(c *gin.Context) {
	agentID := c.Param("id")

	// TODO: Delete agent
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "DeleteAgent endpoint not yet implemented",
		"agent_id": agentID,
	})
}

// SearchAgents searches for agents by capabilities
func (h *Handlers) SearchAgents(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "query parameter 'q' is required",
		})
		return
	}

	// TODO: Implement semantic search using HNSW index
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "SearchAgents endpoint not yet implemented",
	})
}

// Helper functions for WASM validation

// isValidWASM checks if the data is a valid WebAssembly binary
func isValidWASM(data []byte) bool {
	// Check WASM magic bytes: 0x00 0x61 0x73 0x6D (\0asm)
	if len(data) < 8 {
		return false
	}

	magicBytes := []byte{0x00, 0x61, 0x73, 0x6D}
	return bytes.Equal(data[0:4], magicBytes)
}

// calculateWASMHash calculates SHA-256 hash of WASM binary
func calculateWASMHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Request/Response types

// RegisterAgentRequest represents an agent registration request
type RegisterAgentRequest struct {
	Name         string                 `json:"name" binding:"required"`
	Description  string                 `json:"description"`
	Capabilities []string               `json:"capabilities" binding:"required"`
	Pricing      *AgentPricing          `json:"pricing" binding:"required"`
	Resources    *AgentResources        `json:"resources"`
	Metadata     map[string]interface{} `json:"metadata"`
	// WASM binary is uploaded as multipart file
}

// AgentPricing defines agent pricing structure
type AgentPricing struct {
	PerExecution float64 `json:"per_execution" binding:"required,gt=0"`
	PerSecond    float64 `json:"per_second"`
	PerMB        float64 `json:"per_mb"`
	Currency     string  `json:"currency"` // "USD", "tokens", etc.
}

// AgentResources defines agent resource requirements
type AgentResources struct {
	CPULimit    string `json:"cpu_limit"`    // e.g., "500m"
	MemoryLimit string `json:"memory_limit"` // e.g., "128Mi"
	Timeout     int    `json:"timeout"`      // seconds
}

// RegisterAgentResponse represents the registration response
type RegisterAgentResponse struct {
	AgentID       string    `json:"agent_id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	WASMHash      string    `json:"wasm_hash"`
	CardPublished bool      `json:"card_published"`
	IndexUpdated  bool      `json:"index_updated"`
	Timestamp     time.Time `json:"timestamp"`
}

const (
	// Maximum WASM binary sizes
	MaxWASMSize     = 50 * 1024 * 1024 // 50MB
	MinWASMSize     = 1024              // 1KB
	MaxCapabilities = 50
	MaxMetadataSize = 10 * 1024 // 10KB JSON
)
