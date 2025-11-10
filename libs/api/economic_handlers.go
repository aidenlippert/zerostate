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

	// Get user ID from JWT token (if available, otherwise use "anonymous")
	userID := "anonymous"
	if userIDValue, exists := c.Get("user_id"); exists {
		if uid, ok := userIDValue.(string); ok {
			userID = uid
		}
	}

	// Set default priority if not provided
	if req.Priority == "" {
		req.Priority = "normal"
	}

	// Create delegation using real meta-orchestrator service
	metaSvc := economic.NewMetaOrchestratorService(h.db.Conn(), h.logger)
	delegation, subtasks, err := metaSvc.CreateDelegation(
		c.Request.Context(),
		req.TaskID,
		userID,
		req.Query,
		req.Capabilities,
		req.Budget,
		req.Priority,
	)
	if err != nil {
		logger.Error("failed to create delegation",
			zap.String("task_id", req.TaskID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create delegation",
			"message": err.Error(),
		})
		return
	}

	// Convert subtasks to simple descriptions for response
	subtaskDescriptions := make([]string, len(subtasks))
	for i, st := range subtasks {
		subtaskDescriptions[i] = st.Description
	}

	logger.Info("task delegated to meta-orchestrator",
		zap.String("delegation_id", delegation.ID.String()),
		zap.String("task_id", req.TaskID),
		zap.Int("subtasks", len(subtasks)),
	)

	c.JSON(http.StatusCreated, gin.H{
		"delegation_id":        delegation.ID.String(),
		"task_id":              delegation.TaskID,
		"query":                delegation.Query,
		"status":               string(delegation.Status),
		"capabilities":         delegation.Capabilities,
		"budget":               delegation.Budget,
		"priority":             delegation.Priority,
		"agents_count":         delegation.AgentsCount,
		"subtasks":             subtaskDescriptions,
		"created_at":           delegation.CreatedAt.Format(time.RFC3339),
		"estimated_completion": delegation.EstimatedCompletion.Format(time.RFC3339),
	})
}

// GetOrchestrationStatus retrieves status of meta-orchestrator delegation
func (h *Handlers) GetOrchestrationStatus(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetOrchestrationStatus"))
	taskID := c.Param("task_id")

	// Get delegation and subtasks using real meta-orchestrator service
	metaSvc := economic.NewMetaOrchestratorService(h.db.Conn(), h.logger)

	delegation, err := metaSvc.GetDelegationByTaskID(c.Request.Context(), taskID)
	if err != nil {
		logger.Error("delegation not found",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "delegation not found",
			"message": err.Error(),
		})
		return
	}

	// Get subtasks for this delegation
	subtasks, err := metaSvc.GetSubtasks(c.Request.Context(), delegation.ID)
	if err != nil {
		logger.Error("failed to get subtasks",
			zap.String("delegation_id", delegation.ID.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get subtasks",
			"message": err.Error(),
		})
		return
	}

	// Calculate progress
	completedCount := 0
	assignedAgents := make(map[string]bool)
	for _, st := range subtasks {
		if st.Status == economic.SubtaskStatusCompleted {
			completedCount++
		}
		if st.AgentID != nil {
			assignedAgents[*st.AgentID] = true
		}
	}

	agentList := make([]string, 0, len(assignedAgents))
	for agentID := range assignedAgents {
		agentList = append(agentList, agentID)
	}

	progressPercentage := 0.0
	if len(subtasks) > 0 {
		progressPercentage = (float64(completedCount) / float64(len(subtasks))) * 100.0
	}

	logger.Info("orchestration status retrieved",
		zap.String("task_id", taskID),
		zap.String("status", string(delegation.Status)),
		zap.Float64("progress", progressPercentage),
	)

	c.JSON(http.StatusOK, gin.H{
		"delegation_id":        delegation.ID.String(),
		"task_id":              delegation.TaskID,
		"status":               string(delegation.Status),
		"agents_assigned":      agentList,
		"subtasks_completed":   completedCount,
		"subtasks_total":       len(subtasks),
		"progress_percentage":  progressPercentage,
		"estimated_completion": delegation.EstimatedCompletion.Format(time.RFC3339),
		"last_updated":         delegation.UpdatedAt.Format(time.RFC3339),
	})
}

// Escrow Handlers

// CreateEscrow handles escrow creation for task payments
func (h *Handlers) CreateEscrow(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "CreateEscrow"))

	var req struct {
		TaskID              string  `json:"task_id" binding:"required"`
		PayeeID             string  `json:"payee_id" binding:"required"`
		Amount              float64 `json:"amount" binding:"required,gt=0"`
		ExpirationMinutes   int     `json:"expiration_minutes" binding:"required,gt=0"`
		AutoReleaseMinutes  *int    `json:"auto_release_minutes"`
		Conditions          string  `json:"conditions"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Extract user ID from JWT token (payer)
	userID, exists := c.Get("user_id")
	if !exists {
		userID = "anonymous"
	}
	payerID := userID.(string)

	// Create escrow using economic service
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	escrow, err := escrowSvc.CreateEscrow(
		c.Request.Context(),
		req.TaskID,
		payerID,
		req.PayeeID,
		req.Amount,
		req.ExpirationMinutes,
		req.AutoReleaseMinutes,
		req.Conditions,
	)
	if err != nil {
		logger.Error("failed to create escrow", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create escrow",
			"message": err.Error(),
		})
		return
	}

	logger.Info("escrow created",
		zap.String("escrow_id", escrow.ID.String()),
		zap.String("task_id", req.TaskID),
		zap.Float64("amount", req.Amount),
	)

	c.JSON(http.StatusOK, gin.H{
		"escrow_id":    escrow.ID.String(),
		"task_id":      escrow.TaskID,
		"payer_id":     escrow.PayerID,
		"payee_id":     escrow.PayeeID,
		"amount":       escrow.Amount,
		"status":       string(escrow.Status),
		"expires_at":   escrow.ExpiresAt.Format(time.RFC3339),
		"created_at":   escrow.CreatedAt.Format(time.RFC3339),
	})
}

// FundEscrow handles funding an escrow
func (h *Handlers) FundEscrow(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "FundEscrow"))

	escrowIDStr := c.Param("id")
	escrowID, err := uuid.Parse(escrowIDStr)
	if err != nil {
		logger.Error("invalid escrow ID", zap.String("escrow_id", escrowIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid escrow ID",
			"message": err.Error(),
		})
		return
	}

	var req struct {
		Signature string `json:"signature" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Fund escrow
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	err = escrowSvc.FundEscrow(c.Request.Context(), escrowID, req.Signature)
	if err != nil {
		logger.Error("failed to fund escrow", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to fund escrow",
			"message": err.Error(),
		})
		return
	}

	logger.Info("escrow funded", zap.String("escrow_id", escrowID.String()))

	c.JSON(http.StatusOK, gin.H{
		"escrow_id": escrowID.String(),
		"status":    "funded",
		"message":   "escrow successfully funded",
	})
}

// ReleaseEscrow handles releasing escrow funds to payee
func (h *Handlers) ReleaseEscrow(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "ReleaseEscrow"))

	escrowIDStr := c.Param("id")
	escrowID, err := uuid.Parse(escrowIDStr)
	if err != nil {
		logger.Error("invalid escrow ID", zap.String("escrow_id", escrowIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid escrow ID",
			"message": err.Error(),
		})
		return
	}

	// Extract user ID from JWT token
	userID, exists := c.Get("user_id")
	if !exists {
		userID = "anonymous"
	}
	releasedBy := userID.(string)

	// Release escrow
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	err = escrowSvc.ReleaseEscrow(c.Request.Context(), escrowID, releasedBy)
	if err != nil {
		logger.Error("failed to release escrow", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to release escrow",
			"message": err.Error(),
		})
		return
	}

	logger.Info("escrow released",
		zap.String("escrow_id", escrowID.String()),
		zap.String("released_by", releasedBy),
	)

	c.JSON(http.StatusOK, gin.H{
		"escrow_id": escrowID.String(),
		"status":    "released",
		"message":   "escrow successfully released to payee",
	})
}

// RefundEscrow handles refunding escrow funds to payer
func (h *Handlers) RefundEscrow(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "RefundEscrow"))

	escrowIDStr := c.Param("id")
	escrowID, err := uuid.Parse(escrowIDStr)
	if err != nil {
		logger.Error("invalid escrow ID", zap.String("escrow_id", escrowIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid escrow ID",
			"message": err.Error(),
		})
		return
	}

	// Extract user ID from JWT token
	userID, exists := c.Get("user_id")
	if !exists {
		userID = "anonymous"
	}
	refundedBy := userID.(string)

	// Refund escrow
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	err = escrowSvc.RefundEscrow(c.Request.Context(), escrowID, refundedBy)
	if err != nil {
		logger.Error("failed to refund escrow", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to refund escrow",
			"message": err.Error(),
		})
		return
	}

	logger.Info("escrow refunded",
		zap.String("escrow_id", escrowID.String()),
		zap.String("refunded_by", refundedBy),
	)

	c.JSON(http.StatusOK, gin.H{
		"escrow_id": escrowID.String(),
		"status":    "refunded",
		"message":   "escrow successfully refunded to payer",
	})
}

// GetEscrow retrieves escrow details
func (h *Handlers) GetEscrow(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetEscrow"))

	escrowIDStr := c.Param("id")
	escrowID, err := uuid.Parse(escrowIDStr)
	if err != nil {
		logger.Error("invalid escrow ID", zap.String("escrow_id", escrowIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid escrow ID",
			"message": err.Error(),
		})
		return
	}

	// Get escrow
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	escrow, err := escrowSvc.GetEscrow(c.Request.Context(), escrowID)
	if err != nil {
		logger.Error("failed to get escrow", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "escrow not found",
			"message": err.Error(),
		})
		return
	}

	response := gin.H{
		"escrow_id":    escrow.ID.String(),
		"task_id":      escrow.TaskID,
		"payer_id":     escrow.PayerID,
		"payee_id":     escrow.PayeeID,
		"amount":       escrow.Amount,
		"status":       string(escrow.Status),
		"expires_at":   escrow.ExpiresAt.Format(time.RFC3339),
		"created_at":   escrow.CreatedAt.Format(time.RFC3339),
		"updated_at":   escrow.UpdatedAt.Format(time.RFC3339),
	}

	if escrow.FundedAt != nil {
		response["funded_at"] = escrow.FundedAt.Format(time.RFC3339)
	}
	if escrow.ReleasedAt != nil {
		response["released_at"] = escrow.ReleasedAt.Format(time.RFC3339)
	}
	if escrow.RefundedAt != nil {
		response["refunded_at"] = escrow.RefundedAt.Format(time.RFC3339)
	}
	if escrow.AutoReleaseAt != nil {
		response["auto_release_at"] = escrow.AutoReleaseAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, response)
}

// Dispute Handlers

// OpenDispute handles opening a dispute on an escrow
func (h *Handlers) OpenDispute(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "OpenDispute"))

	escrowIDStr := c.Param("id")
	escrowID, err := uuid.Parse(escrowIDStr)
	if err != nil {
		logger.Error("invalid escrow ID", zap.String("escrow_id", escrowIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid escrow ID",
			"message": err.Error(),
		})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Extract user ID from JWT token
	userID, exists := c.Get("user_id")
	if !exists {
		userID = "anonymous"
	}
	initiatorID := userID.(string)

	// Open dispute
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	dispute, err := escrowSvc.OpenDispute(c.Request.Context(), escrowID, initiatorID, req.Reason)
	if err != nil {
		logger.Error("failed to open dispute", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to open dispute",
			"message": err.Error(),
		})
		return
	}

	logger.Info("dispute opened",
		zap.String("dispute_id", dispute.ID.String()),
		zap.String("escrow_id", escrowID.String()),
	)

	c.JSON(http.StatusOK, gin.H{
		"dispute_id":   dispute.ID.String(),
		"escrow_id":    dispute.EscrowID.String(),
		"initiator_id": dispute.InitiatorID,
		"reason":       dispute.Reason,
		"status":       string(dispute.Status),
		"created_at":   dispute.CreatedAt.Format(time.RFC3339),
	})
}

// SubmitEvidence handles submitting evidence for a dispute
func (h *Handlers) SubmitEvidence(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "SubmitEvidence"))

	disputeIDStr := c.Param("id")
	disputeID, err := uuid.Parse(disputeIDStr)
	if err != nil {
		logger.Error("invalid dispute ID", zap.String("dispute_id", disputeIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid dispute ID",
			"message": err.Error(),
		})
		return
	}

	var req struct {
		EvidenceType string  `json:"evidence_type" binding:"required"`
		Content      string  `json:"content" binding:"required"`
		FileURL      *string `json:"file_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Extract user ID from JWT token
	userID, exists := c.Get("user_id")
	if !exists {
		userID = "anonymous"
	}
	submitterID := userID.(string)

	// Submit evidence
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	evidence, err := escrowSvc.SubmitEvidence(
		c.Request.Context(),
		disputeID,
		submitterID,
		req.EvidenceType,
		req.Content,
		req.FileURL,
	)
	if err != nil {
		logger.Error("failed to submit evidence", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to submit evidence",
			"message": err.Error(),
		})
		return
	}

	logger.Info("evidence submitted",
		zap.String("evidence_id", evidence.ID.String()),
		zap.String("dispute_id", disputeID.String()),
	)

	response := gin.H{
		"evidence_id":   evidence.ID.String(),
		"dispute_id":    evidence.DisputeID.String(),
		"submitter_id":  evidence.SubmitterID,
		"evidence_type": evidence.EvidenceType,
		"content":       evidence.Content,
		"created_at":    evidence.CreatedAt.Format(time.RFC3339),
	}

	if evidence.FileURL != nil {
		response["file_url"] = *evidence.FileURL
	}

	c.JSON(http.StatusOK, response)
}

// ResolveDispute handles resolving a dispute
func (h *Handlers) ResolveDispute(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "ResolveDispute"))

	disputeIDStr := c.Param("id")
	disputeID, err := uuid.Parse(disputeIDStr)
	if err != nil {
		logger.Error("invalid dispute ID", zap.String("dispute_id", disputeIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid dispute ID",
			"message": err.Error(),
		})
		return
	}

	var req struct {
		Resolution string `json:"resolution" binding:"required"`
		Outcome    string `json:"outcome" binding:"required,oneof=release refund"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Extract user ID from JWT token (reviewer)
	userID, exists := c.Get("user_id")
	if !exists {
		userID = "system"
	}
	reviewerID := userID.(string)

	// Resolve dispute
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	err = escrowSvc.ResolveDispute(
		c.Request.Context(),
		disputeID,
		reviewerID,
		req.Resolution,
		req.Outcome,
	)
	if err != nil {
		logger.Error("failed to resolve dispute", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to resolve dispute",
			"message": err.Error(),
		})
		return
	}

	logger.Info("dispute resolved",
		zap.String("dispute_id", disputeID.String()),
		zap.String("reviewer_id", reviewerID),
		zap.String("outcome", req.Outcome),
	)

	c.JSON(http.StatusOK, gin.H{
		"dispute_id": disputeID.String(),
		"status":     "resolved",
		"outcome":    req.Outcome,
		"resolution": req.Resolution,
		"message":    "dispute successfully resolved",
	})
}

// GetDispute retrieves dispute details with evidence
func (h *Handlers) GetDispute(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetDispute"))

	disputeIDStr := c.Param("id")
	disputeID, err := uuid.Parse(disputeIDStr)
	if err != nil {
		logger.Error("invalid dispute ID", zap.String("dispute_id", disputeIDStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid dispute ID",
			"message": err.Error(),
		})
		return
	}

	// Get dispute
	escrowSvc := economic.NewEscrowService(h.db.Conn(), h.logger)

	dispute, err := escrowSvc.GetDispute(c.Request.Context(), disputeID)
	if err != nil {
		logger.Error("failed to get dispute", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "dispute not found",
			"message": err.Error(),
		})
		return
	}

	// Get evidence
	evidence, err := escrowSvc.GetDisputeEvidence(c.Request.Context(), disputeID)
	if err != nil {
		logger.Error("failed to get evidence", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get evidence",
			"message": err.Error(),
		})
		return
	}

	response := gin.H{
		"dispute_id":   dispute.ID.String(),
		"escrow_id":    dispute.EscrowID.String(),
		"initiator_id": dispute.InitiatorID,
		"reason":       dispute.Reason,
		"status":       string(dispute.Status),
		"created_at":   dispute.CreatedAt.Format(time.RFC3339),
		"updated_at":   dispute.UpdatedAt.Format(time.RFC3339),
		"evidence":     evidence,
	}

	if dispute.ReviewerID != nil {
		response["reviewer_id"] = *dispute.ReviewerID
	}
	if dispute.Resolution != nil {
		response["resolution"] = *dispute.Resolution
	}
	if dispute.ResolvedAt != nil {
		response["resolved_at"] = dispute.ResolvedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, response)
}
