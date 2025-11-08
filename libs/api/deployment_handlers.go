package api

import (
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DeployAgentRequest represents a request to deploy an agent
type DeployAgentRequest struct {
	AgentID     string `json:"agent_id" binding:"required"`
	Environment string `json:"environment"` // development, staging, production
	Config      string `json:"config"`      // JSON configuration
}

// DeploymentResponse represents a deployment in API responses
type DeploymentResponse struct {
	ID          string `json:"id"`
	AgentID     string `json:"agent_id"`
	UserID      string `json:"user_id"`
	Status      string `json:"status"`
	Environment string `json:"environment"`
	Config      string `json:"config,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// DeployAgent creates a new agent deployment
func (h *Handlers) DeployAgent(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "DeployAgent"))

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req DeployAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "message": err.Error()})
		return
	}

	// Verify agent exists
	agent, err := h.db.GetAgentByID(req.AgentID)
	if err != nil {
		logger.Error("failed to get agent", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if agent == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Set default environment if not specified
	if req.Environment == "" {
		req.Environment = "development"
	}

	// Validate environment
	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[req.Environment] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment", "message": "must be development, staging, or production"})
		return
	}

	// Create deployment
	deployment := &database.AgentDeployment{
		ID:          uuid.New().String(),
		AgentID:     req.AgentID,
		UserID:      userID.(string),
		Status:      "deployed",
		Environment: req.Environment,
		Config:      req.Config,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.db.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, DeploymentResponse{
		ID:          deployment.ID,
		AgentID:     deployment.AgentID,
		UserID:      deployment.UserID,
		Status:      deployment.Status,
		Environment: deployment.Environment,
		Config:      deployment.Config,
		CreatedAt:   deployment.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   deployment.UpdatedAt.Format(time.RFC3339),
	})
}

// GetDeployment retrieves a deployment by ID
func (h *Handlers) GetDeployment(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetDeployment"))

	deploymentID := c.Param("id")

	deployment, err := h.db.GetDeploymentByID(deploymentID)
	if err != nil {
		logger.Error("failed to get deployment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if deployment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}

	c.JSON(http.StatusOK, DeploymentResponse{
		ID:          deployment.ID,
		AgentID:     deployment.AgentID,
		UserID:      deployment.UserID,
		Status:      deployment.Status,
		Environment: deployment.Environment,
		Config:      deployment.Config,
		CreatedAt:   deployment.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   deployment.UpdatedAt.Format(time.RFC3339),
	})
}

// ListUserDeployments lists all deployments for the current user
func (h *Handlers) ListUserDeployments(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "ListUserDeployments"))

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deployments, err := h.db.ListDeploymentsByUser(userID.(string))
	if err != nil {
		logger.Error("failed to list deployments", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	responses := make([]DeploymentResponse, 0, len(deployments))
	for _, d := range deployments {
		responses = append(responses, DeploymentResponse{
			ID:          d.ID,
			AgentID:     d.AgentID,
			UserID:      d.UserID,
			Status:      d.Status,
			Environment: d.Environment,
			Config:      d.Config,
			CreatedAt:   d.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   d.UpdatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"deployments": responses,
		"total":       len(responses),
	})
}

// StopDeployment stops a deployment
func (h *Handlers) StopDeployment(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "StopDeployment"))

	deploymentID := c.Param("id")

	// Get deployment
	deployment, err := h.db.GetDeploymentByID(deploymentID)
	if err != nil {
		logger.Error("failed to get deployment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if deployment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}

	// Verify user owns this deployment
	userID, exists := c.Get("user_id")
	if !exists || deployment.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// Update status to stopped
	deployment.Status = "stopped"
	if err := h.db.UpdateDeployment(deployment); err != nil {
		logger.Error("failed to stop deployment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, DeploymentResponse{
		ID:          deployment.ID,
		AgentID:     deployment.AgentID,
		UserID:      deployment.UserID,
		Status:      deployment.Status,
		Environment: deployment.Environment,
		Config:      deployment.Config,
		CreatedAt:   deployment.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   deployment.UpdatedAt.Format(time.RFC3339),
	})
}
