package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ExecutionHandlers handles task execution and results
type ExecutionHandlers struct {
	logger       *zap.Logger
	db           *database.DB
	wasmRunner   *execution.WASMRunner
	resultStore  *execution.PostgresResultStore
	binaryStore  execution.BinaryStore
	economicExec *execution.EconomicExecutor // Sprint 9: Economic task execution
}

// NewExecutionHandlers creates execution handlers
func NewExecutionHandlers(
	logger *zap.Logger,
	db *database.DB,
	wasmRunner *execution.WASMRunner,
	resultStore *execution.PostgresResultStore,
	binaryStore execution.BinaryStore,
) *ExecutionHandlers {
	return &ExecutionHandlers{
		logger:      logger,
		db:          db,
		wasmRunner:  wasmRunner,
		resultStore: resultStore,
		binaryStore: binaryStore,
	}
}

// ExecuteTaskRequest represents task execution parameters
type ExecuteTaskRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
	Input   string `json:"input"`
}

// ExecuteTaskDirect executes a task immediately and returns result
// POST /api/v1/tasks/execute
func (h *ExecutionHandlers) ExecuteTaskDirect(c *gin.Context) {
	var req ExecuteTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	// Verify agent exists and get binary info
	agent, err := h.db.GetAgentByID(req.AgentID)
	if err != nil {
		h.logger.Error("failed to get agent", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "failed to verify agent",
		})
		return
	}
	if agent == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "agent_not_found",
			"message": fmt.Sprintf("agent %s not found", req.AgentID),
		})
		return
	}

	// Check agent status
	if agent.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "agent_unavailable",
			"message": fmt.Sprintf("agent is %s", agent.Status),
		})
		return
	}

	// Download WASM binary
	ctx := context.Background()
	binary, err := h.binaryStore.GetBinary(ctx, req.AgentID)
	if err != nil {
		h.logger.Error("failed to download binary",
			zap.Error(err),
			zap.String("agent_id", req.AgentID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "binary_error",
			"message": "failed to download agent binary",
		})
		return
	}

	// Execute WASM
	h.logger.Info("executing task",
		zap.String("agent_id", req.AgentID),
		zap.Int("binary_size", len(binary)),
		zap.String("input", req.Input),
	)

	startTime := time.Now()
	result, err := h.wasmRunner.Execute(ctx, binary, []byte(req.Input))
	duration := time.Since(startTime)

	if err != nil {
		h.logger.Error("execution failed",
			zap.Error(err),
			zap.String("agent_id", req.AgentID),
			zap.Duration("duration", duration),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "execution_error",
			"message": err.Error(),
		})
		return
	}

	// Generate task ID
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	// Store result in database
	// Convert error to string for database storage
	var errorStr string
	if result.Error != nil {
		errorStr = result.Error.Error()
	}

	taskResult := &execution.TaskResult{
		TaskID:     taskID,
		AgentID:    req.AgentID,
		ExitCode:   result.ExitCode,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		DurationMs: duration.Milliseconds(),
		Error:      errorStr,
		CreatedAt:  time.Now(),
	}

	if err := h.resultStore.StoreResult(ctx, taskResult); err != nil {
		h.logger.Error("failed to store result",
			zap.Error(err),
			zap.String("task_id", taskID),
		)
		// Don't fail the request - we have the result, just couldn't persist it
	}

	// Update agent stats (increment tasks_completed)
	if result.ExitCode == 0 {
		h.logger.Info("updating agent stats",
			zap.String("agent_id", req.AgentID),
			zap.Int("current_tasks", agent.TasksCompleted),
		)
		// Note: Would need UpdateAgentStats method in database package
	}

	h.logger.Info("task executed successfully",
		zap.String("task_id", taskID),
		zap.String("agent_id", req.AgentID),
		zap.Int("exit_code", result.ExitCode),
		zap.Duration("duration", duration),
	)

	c.JSON(http.StatusOK, gin.H{
		"task_id":     taskID,
		"agent_id":    req.AgentID,
		"exit_code":   result.ExitCode,
		"stdout":      string(result.Stdout),
		"stderr":      string(result.Stderr),
		"duration_ms": duration.Milliseconds(),
		"error":       result.Error,
	})
}

// GetTaskResult retrieves result for a specific task
// GET /api/v1/tasks/:id/results
func (h *ExecutionHandlers) GetTaskResult(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "task_id is required",
		})
		return
	}

	ctx := context.Background()
	result, err := h.resultStore.GetResult(ctx, taskID)
	if err != nil {
		h.logger.Error("failed to get result",
			zap.Error(err),
			zap.String("task_id", taskID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "failed to retrieve task result",
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": fmt.Sprintf("task %s not found", taskID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":     result.TaskID,
		"agent_id":    result.AgentID,
		"exit_code":   result.ExitCode,
		"stdout":      string(result.Stdout),
		"stderr":      string(result.Stderr),
		"duration_ms": result.DurationMs,
		"error":       result.Error,
		"created_at":  result.CreatedAt,
	})
}

// ListTaskResults lists all results (optionally filtered by agent_id)
// GET /api/v1/tasks/results?agent_id=xxx&limit=10&offset=0
func (h *ExecutionHandlers) ListTaskResults(c *gin.Context) {
	agentID := c.Query("agent_id")
	limit := c.DefaultQuery("limit", "10")
	offset := c.DefaultQuery("offset", "0")

	h.logger.Info("listing task results",
		zap.String("agent_id", agentID),
		zap.String("limit", limit),
		zap.String("offset", offset),
	)

	ctx := context.Background()
	results, err := h.resultStore.ListResults(ctx, agentID, limit, offset)
	if err != nil {
		h.logger.Error("failed to list results",
			zap.Error(err),
			zap.String("agent_id", agentID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "failed to list task results",
		})
		return
	}

	// Convert to response format
	response := make([]gin.H, len(results))
	for i, result := range results {
		response[i] = gin.H{
			"task_id":     result.TaskID,
			"agent_id":    result.AgentID,
			"exit_code":   result.ExitCode,
			"stdout":      string(result.Stdout),
			"stderr":      string(result.Stderr),
			"duration_ms": result.DurationMs,
			"error":       result.Error,
			"created_at":  result.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": response,
		"count":   len(results),
		"limit":   limit,
		"offset":  offset,
	})
}
