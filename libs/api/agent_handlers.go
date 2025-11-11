package api

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	// Parse JSON metadata BEFORE reading large file to avoid FormValue() issues
	var req RegisterAgentRequest
	agentJSON := c.Request.FormValue("agent")
	if agentJSON == "" {
		logger.Error("missing agent JSON field")
		if span != nil {
			span.SetStatus(codes.Error, "missing agent field")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "agent field with JSON data is required",
		})
		return
	}

	if err := json.Unmarshal([]byte(agentJSON), &req); err != nil {
		logger.Error("failed to parse agent JSON", zap.Error(err))
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid JSON data")
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": fmt.Sprintf("failed to parse agent JSON: %v", err),
		})
		return
	}

	logger.Info("agent metadata parsed successfully",
		zap.String("name", req.Name),
		zap.Strings("capabilities", req.Capabilities),
	)

	// Now get WASM binary file
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

	// Validate capabilities (req already parsed earlier)
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

	// Save agent to database
	if h.db != nil {
		// Convert capabilities to JSON
		capabilitiesJSON, err := json.Marshal(req.Capabilities)
		if err != nil {
			logger.Error("failed to marshal capabilities", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to process capabilities",
			})
			return
		}
		// Ensure capabilities is never null for PostgreSQL JSONB column (must be [] not null)
		if len(capabilitiesJSON) == 0 || string(capabilitiesJSON) == "null" {
			capabilitiesJSON = json.RawMessage(`[]`)
		}

		// Convert pricing to JSON for PricingModel field
		pricingJSON, err := json.Marshal(req.Pricing)
		if err != nil {
			logger.Error("failed to marshal pricing", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to process pricing",
			})
			return
		}

		// Create metadata with binary hash
		metadata := map[string]interface{}{
			"wasm_hash": wasmHash,
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			logger.Error("failed to marshal metadata", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to process metadata",
			})
			return
		}
		// Ensure metadata is never nil
		if len(metadataJSON) == 0 {
			metadataJSON = json.RawMessage(`{}`)
		}

		// Create agent record
		agentUUID := uuid.New()
		now := time.Now()
		agent := &database.Agent{
			ID:           agentUUID,
			DID:          h.signer.DID(),
			Name:         req.Name,
			Description:  sql.NullString{String: req.Description, Valid: req.Description != ""},
			Capabilities: json.RawMessage(capabilitiesJSON),
			PricingModel: sql.NullString{String: string(pricingJSON), Valid: true},
			Status:       database.AgentStatusOnline, // Agent is online and available
			MaxCapacity:  10,                             // Default capacity
			CurrentLoad:  0,
			CreatedAt:    now,
			UpdatedAt:    now,
			Metadata:     json.RawMessage(metadataJSON),
		}

		logger.Info("attempting to save agent to database",
			zap.String("agent_id", agentUUID.String()),
			zap.String("did", h.signer.DID()),
			zap.String("name", req.Name),
			zap.String("status", string(agent.Status)),
			zap.Int("capabilities_len", len(capabilitiesJSON)),
			zap.Int("metadata_len", len(metadataJSON)),
		)

		if err := h.db.CreateAgent(agent); err != nil {
			logger.Error("failed to save agent to database",
				zap.Error(err),
				zap.String("error_type", fmt.Sprintf("%T", err)),
				zap.String("error_detail", err.Error()),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to save agent",
			})
			return
		}

		logger.Info("agent saved to database",
			zap.String("agent_id", h.signer.DID()),
			zap.String("db_id", agentUUID.String()),
		)
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

	// Seed database if empty
	if err := h.seedAgentsIfEmpty(); err != nil {
		logger.Error("failed to seed agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to initialize agent data",
		})
		return
	}

	// Get agent from database
	agent, err := h.db.GetAgentByID(agentID)
	if err != nil {
		logger.Error("failed to get agent", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to retrieve agent",
		})
		return
	}

	if agent == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not found",
			"message": "agent not found",
		})
		return
	}

	// Parse capabilities JSON
	var capabilities []string
	if err := json.Unmarshal([]byte(agent.Capabilities), &capabilities); err != nil {
		logger.Error("failed to parse capabilities", zap.Error(err))
		capabilities = []string{}
	}

	logger.Info("retrieved agent")
	c.JSON(http.StatusOK, gin.H{
		"id":              agent.ID,
		"name":            agent.Name,
		"description":     agent.Description,
		"capabilities":    capabilities,
		"status":          agent.Status,
		"price":           agent.Price,
		"tasks_completed": agent.TasksCompleted,
		"rating":          agent.Rating,
		"created_at":      agent.CreatedAt.Format(time.RFC3339),
		"updated_at":      agent.UpdatedAt.Format(time.RFC3339),
	})
}

// seedAgentsIfEmpty seeds the database with mock agents if empty
func (h *Handlers) seedAgentsIfEmpty() error {
	if h.db == nil {
		// If no database, use mock data
		return nil
	}

	count, err := h.db.GetAgentCount()
	if err != nil {
		return fmt.Errorf("failed to get agent count: %w", err)
	}

	// Database already has agents
	if count > 0 {
		return nil
	}

	// Seed with mock agents
	mockAgents := h.getMockAgents()
	for _, mockAgent := range mockAgents {
		// Marshal capabilities to JSON
		capabilities, _ := json.Marshal(mockAgent["capabilities"])

		// Parse time strings
		createdAt, _ := time.Parse(time.RFC3339, mockAgent["created_at"].(string))
		if createdAt.IsZero() {
			createdAt = time.Now()
		}

		// Convert string ID to uuid.UUID
		agentID, _ := uuid.Parse(mockAgent["id"].(string))

		agent := &database.Agent{
			ID:             agentID,
			DID:            mockAgent["id"].(string),
			Name:           mockAgent["name"].(string),
			Description:    sql.NullString{String: mockAgent["description"].(string), Valid: true},
			Capabilities:   json.RawMessage(capabilities),
			Status:         database.AgentStatus(mockAgent["status"].(string)),
			Price:          mockAgent["price"].(float64),
			TasksCompleted: mockAgent["tasks_completed"].(int),
			Rating:         mockAgent["rating"].(float64),
			CreatedAt:      createdAt,
			UpdatedAt:      time.Now(),
			Metadata:       json.RawMessage(`{}`),
		}

		if err := h.db.CreateAgent(agent); err != nil {
			return fmt.Errorf("failed to create agent %s: %w", agent.ID, err)
		}
	}

	return nil
}

// getMockAgents returns mock agent data for testing
func (h *Handlers) getMockAgents() []gin.H {
	return []gin.H{
		{
			"id":              "agent_001",
			"name":            "DataWeaver",
			"description":     "Advanced data analysis agent for ETL processing and database management",
			"capabilities":    []string{"data_analysis", "etl", "database"},
			"status":          "active",
			"price":           0.02,
			"tasks_completed": 1200000,
			"rating":          4.9,
			"created_at":      time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_002",
			"name":            "Synth-Net",
			"description":     "API integration and data synchronization specialist",
			"capabilities":    []string{"api", "sync", "integration"},
			"status":          "active",
			"price":           0.05,
			"tasks_completed": 890000,
			"rating":          4.8,
			"created_at":      time.Now().Add(-20 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_003",
			"name":            "Code-Gen X",
			"description":     "Automated code generation and refactoring assistant",
			"capabilities":    []string{"code_gen", "refactor", "testing"},
			"status":          "active",
			"price":           25.00,
			"tasks_completed": 450000,
			"rating":          4.7,
			"created_at":      time.Now().Add(-15 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_004",
			"name":            "Orchestrator Prime",
			"description":     "Meta-orchestration agent for complex multi-agent workflows",
			"capabilities":    []string{"orchestration", "workflow", "coordination"},
			"status":          "active",
			"price":           0.10,
			"tasks_completed": 2100000,
			"rating":          4.9,
			"created_at":      time.Now().Add(-45 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_005",
			"name":            "Sentinel",
			"description":     "Security monitoring and threat detection specialist",
			"capabilities":    []string{"security", "monitoring", "threat_detection"},
			"status":          "active",
			"price":           50.00,
			"tasks_completed": 720000,
			"rating":          4.6,
			"created_at":      time.Now().Add(-25 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_006",
			"name":            "Canvas AI",
			"description":     "Image generation and manipulation specialist using DALL-E",
			"capabilities":    []string{"image_gen", "design", "creative"},
			"status":          "active",
			"price":           0.25,
			"tasks_completed": 950000,
			"rating":          4.8,
			"created_at":      time.Now().Add(-18 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_007",
			"name":            "TextCraft Pro",
			"description":     "NLP and content generation specialist powered by GPT-4",
			"capabilities":    []string{"nlp", "text_gen", "summarization"},
			"status":          "active",
			"price":           0.03,
			"tasks_completed": 1800000,
			"rating":          4.7,
			"created_at":      time.Now().Add(-35 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_008",
			"name":            "VoiceForge",
			"description":     "Speech synthesis and voice cloning agent",
			"capabilities":    []string{"tts", "voice_clone", "audio"},
			"status":          "active",
			"price":           0.15,
			"tasks_completed": 580000,
			"rating":          4.5,
			"created_at":      time.Now().Add(-12 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_009",
			"name":            "VideoMorph",
			"description":     "Video processing and editing automation specialist",
			"capabilities":    []string{"video_processing", "editing", "encoding"},
			"status":          "active",
			"price":           0.50,
			"tasks_completed": 320000,
			"rating":          4.6,
			"created_at":      time.Now().Add(-22 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_010",
			"name":            "WebCrawler Elite",
			"description":     "Advanced web scraping and data extraction agent",
			"capabilities":    []string{"web_scraping", "data_extraction", "crawling"},
			"status":          "active",
			"price":           0.08,
			"tasks_completed": 1100000,
			"rating":          4.7,
			"created_at":      time.Now().Add(-28 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_011",
			"name":            "ML Trainer",
			"description":     "Machine learning model training and optimization specialist",
			"capabilities":    []string{"ml_training", "optimization", "model_tuning"},
			"status":          "active",
			"price":           100.00,
			"tasks_completed": 180000,
			"rating":          4.8,
			"created_at":      time.Now().Add(-40 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_012",
			"name":            "CloudSync Master",
			"description":     "Multi-cloud storage synchronization and backup agent",
			"capabilities":    []string{"cloud_storage", "backup", "sync"},
			"status":          "active",
			"price":           0.01,
			"tasks_completed": 2500000,
			"rating":          4.9,
			"created_at":      time.Now().Add(-50 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_013",
			"name":            "BlockChain Oracle",
			"description":     "Blockchain interaction and smart contract deployment agent",
			"capabilities":    []string{"blockchain", "smart_contracts", "web3"},
			"status":          "active",
			"price":           0.20,
			"tasks_completed": 420000,
			"rating":          4.4,
			"created_at":      time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_014",
			"name":            "Quantum Simulator",
			"description":     "Quantum algorithm simulation and optimization",
			"capabilities":    []string{"quantum", "simulation", "optimization"},
			"status":          "beta",
			"price":           5.00,
			"tasks_completed": 85000,
			"rating":          4.2,
			"created_at":      time.Now().Add(-5 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":              "agent_015",
			"name":            "DevOps Automator",
			"description":     "CI/CD pipeline automation and infrastructure as code specialist",
			"capabilities":    []string{"devops", "ci_cd", "infrastructure"},
			"status":          "active",
			"price":           0.12,
			"tasks_completed": 980000,
			"rating":          4.8,
			"created_at":      time.Now().Add(-32 * 24 * time.Hour).Format(time.RFC3339),
		},
	}
}

// ListAgents lists all agents with pagination
func (h *Handlers) ListAgents(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "ListAgents"))

	// Seed database if empty
	if err := h.seedAgentsIfEmpty(); err != nil {
		logger.Error("failed to seed agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to initialize agent data",
		})
		return
	}

	// Get agents from database
	agentsFromDB, err := h.db.ListAgents()
	if err != nil {
		logger.Error("failed to list agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to retrieve agents",
		})
		return
	}

	// Convert database agents to response format
	agents := make([]gin.H, 0, len(agentsFromDB))
	for _, agent := range agentsFromDB {
		var capabilities []string
		if err := json.Unmarshal([]byte(agent.Capabilities), &capabilities); err != nil {
			logger.Error("failed to parse capabilities", zap.Error(err))
			capabilities = []string{}
		}

		agents = append(agents, gin.H{
			"id":              agent.ID,
			"name":            agent.Name,
			"description":     agent.Description,
			"capabilities":    capabilities,
			"status":          agent.Status,
			"price":           agent.Price,
			"tasks_completed": agent.TasksCompleted,
			"rating":          agent.Rating,
			"created_at":      agent.CreatedAt.Format(time.RFC3339),
		})
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
		"error":    "not implemented",
		"message":  "UpdateAgent endpoint not yet implemented",
		"agent_id": agentID,
	})
}

// DeleteAgent deletes an agent
func (h *Handlers) DeleteAgent(c *gin.Context) {
	agentID := c.Param("id")

	// TODO: Delete agent
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":    "not implemented",
		"message":  "DeleteAgent endpoint not yet implemented",
		"agent_id": agentID,
	})
}

// SearchAgents searches for agents by capabilities
func (h *Handlers) SearchAgents(c *gin.Context) {
	query := c.Query("q")
	logger := h.logger.With(zap.String("handler", "SearchAgents"), zap.String("query", query))

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "query parameter 'q' is required",
		})
		return
	}

	// Seed database if empty
	if err := h.seedAgentsIfEmpty(); err != nil {
		logger.Error("failed to seed agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to initialize agent data",
		})
		return
	}

	// For now, implement simple text matching on capabilities and names
	// TODO: Use HNSW index for semantic search when embeddings are available
	logger.Info("searching agents")

	// Search agents in database
	agentsFromDB, err := h.db.SearchAgents(query)
	if err != nil {
		logger.Error("failed to search agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to search agents",
		})
		return
	}

	// Convert database agents to response format
	matchedAgents := make([]gin.H, 0, len(agentsFromDB))
	for _, agent := range agentsFromDB {
		var capabilities []string
		if err := json.Unmarshal([]byte(agent.Capabilities), &capabilities); err != nil {
			logger.Error("failed to parse capabilities", zap.Error(err))
			capabilities = []string{}
		}

		matchedAgents = append(matchedAgents, gin.H{
			"id":              agent.ID,
			"name":            agent.Name,
			"description":     agent.Description,
			"capabilities":    capabilities,
			"status":          agent.Status,
			"price":           agent.Price,
			"tasks_completed": agent.TasksCompleted,
			"rating":          agent.Rating,
			"created_at":      agent.CreatedAt.Format(time.RFC3339),
		})
	}

	logger.Info("search completed",
		zap.Int("matched", len(matchedAgents)),
	)

	c.JSON(http.StatusOK, gin.H{
		"agents": matchedAgents,
		"total":  len(matchedAgents),
		"query":  query,
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
	MinWASMSize     = 1024             // 1KB
	MaxCapabilities = 50
	MaxMetadataSize = 10 * 1024 // 10KB JSON
)
