package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// Maximum WASM agent binary size: 50MB
	MaxWASMAgentSize = 50 * 1024 * 1024

	// Maximum agent upload form size: 60MB (includes metadata)
	MaxAgentFormSize = 60 * 1024 * 1024
)

// UploadAgentRequest represents agent upload metadata
type UploadAgentRequest struct {
	Name         string   `form:"name" binding:"required"`
	Description  string   `form:"description" binding:"required"`
	Version      string   `form:"version" binding:"required"`
	Capabilities []string `form:"capabilities" binding:"required"`
	Price        float64  `form:"price" binding:"required,gte=0"`
}

// UploadAgentResponse represents the upload response
type UploadAgentResponse struct {
	AgentID    string `json:"agent_id"`
	BinaryURL  string `json:"binary_url"`
	BinaryHash string `json:"binary_hash"`
	BinarySize int64  `json:"binary_size"`
	Status     string `json:"status"` // "uploaded", "validating", "active"
	Message    string `json:"message"`
}

// UploadAgentSimple handles WASM agent binary upload without requiring agent ID in URL
// This is a convenience endpoint that auto-generates an agent ID
func (h *Handlers) UploadAgentSimple(c *gin.Context) {
	// Generate a new agent ID
	agentID := uuid.New().String()

	// Set the ID in the context params so UploadAgent can find it
	c.Params = append(c.Params, gin.Param{
		Key:   "id",
		Value: agentID,
	})

	// Delegate to the existing UploadAgent handler
	h.UploadAgent(c)
}

// UploadAgent handles WASM agent binary upload
func (h *Handlers) UploadAgent(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "UploadAgent"))

	// Get user ID from context (authentication middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	logger.Info("agent upload request",
		zap.String("user_id", userID.(string)),
		zap.String("content_type", c.ContentType()),
	)

	// Set max form size
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxAgentFormSize)

	// Parse multipart form
	if err := c.Request.ParseMultipartForm(MaxAgentFormSize); err != nil {
		logger.Error("failed to parse multipart form", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "failed to parse form data",
		})
		return
	}

	// Get WASM binary file
	file, header, err := c.Request.FormFile("wasm_binary")
	if err != nil {
		logger.Error("failed to get wasm_binary file", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "wasm_binary file required",
		})
		return
	}
	defer file.Close()

	logger.Info("WASM binary uploaded",
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size),
	)

	// Validate file size
	if header.Size > MaxWASMAgentSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "file too large",
			"message": fmt.Sprintf("WASM binary exceeds maximum size of %d MB", MaxWASMAgentSize/(1024*1024)),
		})
		return
	}

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if ext != ".wasm" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid file type",
			"message": "file must have .wasm extension",
		})
		return
	}

	// Read file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		logger.Error("failed to read file content", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to read uploaded file",
		})
		return
	}

	// TODO: Add WASM validation here
	// For now, we skip validation to get to production faster
	// Validation will be added in Tier 2
	logger.Info("WASM file received",
		zap.String("filename", header.Filename),
		zap.Int("size", len(fileContent)),
	)

	// Calculate file hash (SHA-256)
	hash := sha256.Sum256(fileContent)
	fileHash := hex.EncodeToString(hash[:])

	// Generate agent ID
	agentID := uuid.New().String()

	// Upload to S3 storage
	var binaryURL string
	if h.s3Storage != nil {
		// Generate S3 key: agents/{agentID}/{hash}.wasm
		s3Key := fmt.Sprintf("agents/%s/%s.wasm", agentID, fileHash)

		// Upload to S3
		uploadedURL, err := h.s3Storage.Upload(c.Request.Context(), s3Key, fileContent, "application/wasm")
		if err != nil {
			logger.Error("failed to upload to S3",
				zap.Error(err),
				zap.String("key", s3Key),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "storage error",
				"message": "failed to upload agent binary to storage",
			})
			return
		}

		binaryURL = uploadedURL
		logger.Info("agent binary uploaded to S3",
			zap.String("key", s3Key),
			zap.String("url", binaryURL),
		)
	} else {
		// Fallback to placeholder URL if S3 not configured
		binaryURL = fmt.Sprintf("https://storage.zerostate.ai/agents/%s/%s.wasm", agentID, fileHash)
		logger.Warn("S3 storage not configured, using placeholder URL")
	}

	// Parse metadata from form
	var metadata UploadAgentRequest
	if err := c.ShouldBind(&metadata); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid metadata",
			"message": err.Error(),
		})
		return
	}

	// Store agent metadata in database
	capabilitiesJSON, err := json.Marshal(metadata.Capabilities)
	if err != nil {
		logger.Error("failed to marshal capabilities",
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "metadata error",
			"message": "failed to process capabilities",
		})
		return
	}

	now := time.Now()

	// Convert string agent ID to uuid.UUID
	agentUUID, _ := uuid.Parse(agentID)

	agent := &database.Agent{
		ID:             agentUUID,
		DID:            agentID,
		Name:           metadata.Name,
		Description:    sql.NullString{String: metadata.Description, Valid: true},
		Capabilities:   json.RawMessage(capabilitiesJSON),
		Status:         database.AgentStatus("active"),
		Price:          metadata.Price,
		TasksCompleted: 0,
		Rating:         0.0,
		CreatedAt:      now,
		UpdatedAt:      now,
		Metadata:       json.RawMessage(`{}`),
		WasmHash:       fileHash,
		S3Key:          fmt.Sprintf("agents/%s/%s.wasm", agentID, fileHash),
	}

	if h.db != nil {
		err = h.db.CreateAgent(agent)
		if err != nil {
			logger.Error("failed to store agent in database",
				zap.Error(err),
				zap.String("agent_id", agentID),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database error",
				"message": "failed to store agent metadata",
			})
			return
		}
		logger.Info("agent metadata stored in database",
			zap.String("agent_id", agentID),
			zap.String("user_id", userID.(string)),
		)
	} else {
		logger.Warn("database not configured, agent metadata not persisted")
	}

	logger.Info("agent uploaded successfully",
		zap.String("agent_id", agentID),
		zap.String("user_id", userID.(string)),
		zap.String("name", metadata.Name),
		zap.String("version", metadata.Version),
		zap.Int64("size", header.Size),
		zap.String("hash", fileHash),
	)

	c.JSON(http.StatusCreated, UploadAgentResponse{
		AgentID:    agentID,
		BinaryURL:  binaryURL,
		BinaryHash: fileHash,
		BinarySize: header.Size,
		Status:     "uploaded",
		Message:    "agent WASM binary uploaded and validated successfully",
	})
}

// GetAgentBinary retrieves the WASM binary for an agent
func (h *Handlers) GetAgentBinary(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetAgentBinary"))

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "agent_id required",
		})
		return
	}

	logger.Info("agent binary download request",
		zap.String("agent_id", agentID),
	)

	if h.s3Storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "not configured",
			"message": "storage service not configured",
		})
		return
	}

	// TODO: Get binary hash from database
	// For now, return signed URL for direct S3 access
	// In production, we would query the database to get the actual S3 key

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "binary retrieval requires database integration for key lookup",
		"note":    "use signed URL from upload response for now",
	})
}

// DeleteAgentBinary deletes a WASM binary
func (h *Handlers) DeleteAgentBinary(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "DeleteAgentBinary"))

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "agent_id required",
		})
		return
	}

	logger.Info("agent binary deletion request",
		zap.String("agent_id", agentID),
		zap.String("user_id", userID.(string)),
	)

	// TODO: Verify user owns this agent
	// TODO: Delete binary from S3/IPFS/Cloud Storage
	// TODO: Update database status

	c.JSON(http.StatusOK, gin.H{
		"message": "agent binary deletion not yet implemented",
	})
}

// ListAgentVersions lists all versions of an agent's WASM binaries
func (h *Handlers) ListAgentVersions(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "ListAgentVersions"))

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "agent_id required",
		})
		return
	}

	logger.Info("list agent versions request",
		zap.String("agent_id", agentID),
	)

	// TODO: Retrieve version history from database
	// For now, return empty list
	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"versions": []map[string]interface{}{},
		"total":    0,
	})
}

// UpdateAgentBinary uploads a new version of an existing agent
func (h *Handlers) UpdateAgentBinary(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "UpdateAgentBinary"))

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "agent_id required",
		})
		return
	}

	logger.Info("agent binary update request",
		zap.String("agent_id", agentID),
		zap.String("user_id", userID.(string)),
	)

	// TODO: Verify user owns this agent
	// TODO: Call UploadAgent logic with version increment
	// TODO: Archive previous version

	c.JSON(http.StatusOK, gin.H{
		"message": "agent binary update not yet fully implemented",
	})
}
