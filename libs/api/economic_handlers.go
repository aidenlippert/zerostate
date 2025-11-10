package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Auction Handlers

// CreateAuction handles auction creation for task assignment
func (h *Handlers) CreateAuction(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "CreateAuction"))

	var req struct {
		TaskID      string  `json:"task_id" binding:"required"`
		MinBid      float64 `json:"min_bid" binding:"required,gt=0"`
		Duration    int     `json:"duration" binding:"required,gt=0"` // seconds
		Description string  `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Create auction using economic service
	econSvc := economic.NewEconomicService(h.db)
	capabilities, _ := json.Marshal([]string{})

	auction, err := econSvc.CreateAuction(c.Request.Context(), req.TaskID, "user_id",
		economic.AuctionTypeFirstPrice, req.Duration, &req.MinBid, nil, nil, capabilities)
	if err != nil {
		logger.Error("failed to create auction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create auction",
			"message": err.Error(),
		})
		return
	}

	logger.Info("auction created",
		zap.String("auction_id", auction.ID.String()),
		zap.String("task_id", req.TaskID),
		zap.Float64("min_bid", req.MinBid),
	)

	c.JSON(http.StatusCreated, gin.H{
		"auction_id":  auction.ID.String(),
		"task_id":     req.TaskID,
		"min_bid":     req.MinBid,
		"status":      auction.Status,
		"bids_count":  0,
		"expires_at":  auction.ExpiresAt.Format(time.RFC3339),
		"created_at":  auction.CreatedAt.Format(time.RFC3339),
		"description": req.Description,
	})
}

// SubmitBid handles bid submission for auctions
func (h *Handlers) SubmitBid(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "SubmitBid"))
	auctionIDStr := c.Param("id")

	auctionID, err := uuid.Parse(auctionIDStr)
	if err != nil {
		logger.Error("invalid auction ID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid auction ID",
			"message": err.Error(),
		})
		return
	}

	var req struct {
		AgentID string  `json:"agent_id" binding:"required"`
		Amount  float64 `json:"amount" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Submit bid using economic service
	econSvc := economic.NewEconomicService(h.db)
	bid, err := econSvc.SubmitBid(c.Request.Context(), auctionID, req.AgentID, req.Amount, nil)
	if err != nil {
		logger.Error("failed to submit bid", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to submit bid",
			"message": err.Error(),
		})
		return
	}

	logger.Info("bid submitted",
		zap.String("bid_id", bid.ID.String()),
		zap.String("auction_id", auctionIDStr),
		zap.String("agent_id", req.AgentID),
		zap.Float64("amount", req.Amount),
	)

	c.JSON(http.StatusCreated, gin.H{
		"bid_id":     bid.ID.String(),
		"auction_id": auctionIDStr,
		"agent_id":   req.AgentID,
		"amount":     req.Amount,
		"status":     "submitted",
		"created_at": bid.CreatedAt.Format(time.RFC3339),
	})
}

// Payment Channel Handlers

// OpenPaymentChannel creates a new payment channel
func (h *Handlers) OpenPaymentChannel(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "OpenPaymentChannel"))

	var req struct {
		AgentID       string  `json:"agent_id" binding:"required"`
		InitialAmount float64 `json:"initial_amount" binding:"required,gt=0"`
		Duration      int     `json:"duration"` // seconds, optional
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Open payment channel using economic service
	econSvc := economic.NewEconomicService(h.db)
	channel, err := econSvc.OpenPaymentChannel(c.Request.Context(), "payer_did", req.AgentID, req.InitialAmount, nil)
	if err != nil {
		logger.Error("failed to open payment channel", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to open payment channel",
			"message": err.Error(),
		})
		return
	}

	logger.Info("payment channel opened",
		zap.String("channel_id", channel.ID.String()),
		zap.String("agent_id", req.AgentID),
		zap.Float64("initial_amount", req.InitialAmount),
	)

	c.JSON(http.StatusCreated, gin.H{
		"channel_id":     channel.ID.String(),
		"agent_id":       req.AgentID,
		"initial_amount": req.InitialAmount,
		"balance":        channel.CurrentBalance,
		"status":         channel.State,
		"nonce":          channel.SequenceNumber,
		"opened_at":      channel.CreatedAt.Format(time.RFC3339),
		"expires_at":     channel.CreatedAt.Add(24 * time.Hour).Format(time.RFC3339),
	})
}

// SettlePaymentChannel closes and settles a payment channel
func (h *Handlers) SettlePaymentChannel(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "SettlePaymentChannel"))
	channelIDStr := c.Param("id")

	channelID, err := uuid.Parse(channelIDStr)
	if err != nil {
		logger.Error("invalid channel ID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid channel ID",
			"message": err.Error(),
		})
		return
	}

	var req struct {
		FinalAmount float64 `json:"final_amount" binding:"required,gte=0"`
		Signature   string  `json:"signature"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Settle payment channel using economic service
	econSvc := economic.NewEconomicService(h.db)
	err = econSvc.SettlePaymentChannel(c.Request.Context(), channelID, req.FinalAmount)
	if err != nil {
		logger.Error("failed to settle payment channel", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to settle payment channel",
			"message": err.Error(),
		})
		return
	}

	settlementID := uuid.New().String()
	settledAt := time.Now()

	logger.Info("payment channel settled",
		zap.String("channel_id", channelIDStr),
		zap.String("settlement_id", settlementID),
		zap.Float64("final_amount", req.FinalAmount),
	)

	c.JSON(http.StatusOK, gin.H{
		"settlement_id": settlementID,
		"channel_id":    channelIDStr,
		"final_amount":  req.FinalAmount,
		"status":        "settled",
		"settled_at":    settledAt.Format(time.RFC3339),
	})
}

// Reputation Handlers

// GetAgentReputation retrieves reputation score for an agent
func (h *Handlers) GetAgentReputation(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetAgentReputation"))
	agentID := c.Param("agent_id")

	// Get reputation using economic service
	econSvc := economic.NewEconomicService(h.db)
	reputation, err := econSvc.GetAgentReputation(c.Request.Context(), agentID)
	if err != nil {
		logger.Error("failed to get reputation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get reputation",
			"message": err.Error(),
		})
		return
	}

	successRate := 0.0
	if reputation.TotalTasks > 0 {
		successRate = float64(reputation.SuccessfulTasks) / float64(reputation.TotalTasks)
	}

	logger.Info("reputation retrieved",
		zap.String("agent_id", agentID),
		zap.Float64("score", reputation.OverallScore),
	)

	c.JSON(http.StatusOK, gin.H{
		"agent_id":          agentID,
		"reputation_score":  reputation.OverallScore,
		"tasks_completed":   reputation.TotalTasks,
		"tasks_successful":  reputation.SuccessfulTasks,
		"success_rate":      successRate,
		"avg_response_time": 250.0, // TODO: Calculate from events
		"uptime":            99.5,  // TODO: Calculate from agent status
		"user_ratings":      []float64{},
		"avg_rating":        reputation.OverallScore / 20.0, // Convert 0-100 to 0-5 scale
		"last_updated":      reputation.UpdatedAt.Format(time.RFC3339),
	})
}

// UpdateAgentReputation updates reputation based on task completion
func (h *Handlers) UpdateAgentReputation(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "UpdateAgentReputation"))

	var req struct {
		AgentID      string  `json:"agent_id" binding:"required"`
		TaskID       string  `json:"task_id" binding:"required"`
		Success      bool    `json:"success"`
		Rating       float64 `json:"rating" binding:"gte=0,lte=5"`
		ResponseTime float64 `json:"response_time"` // milliseconds
		UserFeedback string  `json:"user_feedback"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Update reputation using economic service
	econSvc := economic.NewEconomicService(h.db)
	err := econSvc.UpdateAgentReputation(c.Request.Context(), req.AgentID, req.TaskID,
		req.Success, req.Rating, int(req.ResponseTime))
	if err != nil {
		logger.Error("failed to update reputation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to update reputation",
			"message": err.Error(),
		})
		return
	}

	// Get updated reputation
	reputation, err := econSvc.GetAgentReputation(c.Request.Context(), req.AgentID)
	if err != nil {
		logger.Error("failed to get updated reputation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get updated reputation",
			"message": err.Error(),
		})
		return
	}

	successRate := 0.0
	if reputation.TotalTasks > 0 {
		successRate = float64(reputation.SuccessfulTasks) / float64(reputation.TotalTasks)
	}

	logger.Info("reputation updated",
		zap.String("agent_id", req.AgentID),
		zap.String("task_id", req.TaskID),
		zap.Bool("success", req.Success),
		zap.Float64("new_score", reputation.OverallScore),
	)

	c.JSON(http.StatusOK, gin.H{
		"agent_id":          req.AgentID,
		"task_id":           req.TaskID,
		"reputation_score":  reputation.OverallScore,
		"tasks_completed":   reputation.TotalTasks,
		"tasks_successful":  reputation.SuccessfulTasks,
		"success_rate":      successRate,
		"rating_submitted":  req.Rating,
		"response_time":     req.ResponseTime,
		"updated_at":        time.Now().Format(time.RFC3339),
	})
}

// Meta-Orchestrator Handlers

// DelegateToMetaOrchestrator delegates a complex task to meta-orchestrator
func (h *Handlers) DelegateToMetaOrchestrator(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "DelegateToMetaOrchestrator"))

	var req struct {
		TaskID       string                 `json:"task_id" binding:"required"`
		Query        string                 `json:"query" binding:"required"`
		Capabilities []string               `json:"capabilities"`
		Budget       float64                `json:"budget" binding:"required,gt=0"`
		Priority     string                 `json:"priority"`
		Metadata     map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// TODO: Implement real meta-orchestrator logic
	// For now, return mock data indicating delegated status
	delegationID := uuid.New().String()
	createdAt := time.Now()

	logger.Info("task delegated to meta-orchestrator",
		zap.String("delegation_id", delegationID),
		zap.String("task_id", req.TaskID),
		zap.String("query", req.Query),
		zap.Float64("budget", req.Budget),
	)

	c.JSON(http.StatusCreated, gin.H{
		"delegation_id": delegationID,
		"task_id":       req.TaskID,
		"query":         req.Query,
		"status":        "delegated",
		"capabilities":  req.Capabilities,
		"budget":        req.Budget,
		"priority":      req.Priority,
		"agents_count":  0,
		"subtasks":      []string{},
		"created_at":    createdAt.Format(time.RFC3339),
		"estimated_completion": time.Now().Add(5 * time.Minute).Format(time.RFC3339),
	})
}

// GetOrchestrationStatus retrieves status of meta-orchestrator delegation
func (h *Handlers) GetOrchestrationStatus(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetOrchestrationStatus"))
	taskID := c.Param("task_id")

	// TODO: Implement real meta-orchestrator status retrieval
	// For now, return mock status data
	logger.Info("orchestration status requested",
		zap.String("task_id", taskID),
	)

	c.JSON(http.StatusOK, gin.H{
		"task_id":              taskID,
		"status":               "in_progress",
		"agents_assigned":      []string{"agent_001", "agent_002"},
		"subtasks_completed":   1,
		"subtasks_total":       3,
		"progress_percentage":  33.33,
		"estimated_completion": time.Now().Add(3 * time.Minute).Format(time.RFC3339),
		"last_updated":         time.Now().Format(time.RFC3339),
	})
}
