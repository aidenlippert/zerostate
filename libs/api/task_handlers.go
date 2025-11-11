package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SubmitTaskRequest represents a task submission request
type SubmitTaskRequest struct {
	Query        string                 `json:"query" binding:"required"`
	Capabilities []string               `json:"capabilities"` // Agent capabilities required for this task
	Constraints  map[string]interface{} `json:"constraints"`
	Budget       float64                `json:"budget" binding:"required,gt=0"`
	Timeout      int                    `json:"timeout"` // seconds
	Priority     string                 `json:"priority"` // "low", "medium", "high"
}

// SubmitTaskResponse represents the task submission response
type SubmitTaskResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// TaskStatusResponse represents the task status
type TaskStatusResponse struct {
	TaskID     string                 `json:"task_id"`
	Status     string                 `json:"status"` // "queued", "assigned", "running", "completed", "failed"
	Progress   int                    `json:"progress"` // 0-100
	AssignedTo string                 `json:"assigned_to,omitempty"`
	Message    string                 `json:"message,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// TaskResultResponse represents the task result
type TaskResultResponse struct {
	TaskID    string                 `json:"task_id"`
	Status    string                 `json:"status"`
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Cost      float64                `json:"cost"`
	Duration  int                    `json:"duration"` // milliseconds
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SubmitTask handles task submission
func (h *Handlers) SubmitTask(c *gin.Context) {
	ctx := c.Request.Context()
	_ = ctx // For future tracing support
	logger := h.logger.With(zap.String("handler", "SubmitTask"))

	logger.Info("task submission request received",
		zap.String("client_ip", c.ClientIP()),
	)

	// Parse request
	var req SubmitTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("failed to parse request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	// Validate constraints
	if req.Budget <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "budget must be greater than 0",
		})
		return
	}

	// Set default timeout
	if req.Timeout == 0 {
		req.Timeout = 30 // 30 seconds default
	}

	if req.Timeout > 300 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "timeout cannot exceed 300 seconds",
		})
		return
	}

	// Parse priority
	priority := parsePriority(req.Priority)

	// TODO: Extract user ID from authentication context
	// For now, use a placeholder
	userID := "user-" + c.ClientIP()

	// Use capabilities from request, or default to query-processing
	capabilities := req.Capabilities
	if len(capabilities) == 0 {
		capabilities = []string{"query-processing"} // Default for backward compatibility
	}

	// Create task
	task := orchestration.NewTask(
		userID,
		"general-query", // Task type
		capabilities,
		map[string]interface{}{
			"query": req.Query,
			"constraints": req.Constraints,
		},
	)

	task.Priority = priority
	task.Budget = req.Budget
	task.Timeout = time.Duration(req.Timeout) * time.Second

	// Enqueue task
	if h.taskQueue == nil {
		logger.Error("task queue not initialized")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service unavailable",
			"message": "task queue not available",
		})
		return
	}

	if err := h.taskQueue.Enqueue(task); err != nil {
		logger.Error("failed to enqueue task",
			zap.Error(err),
			zap.String("task_id", task.ID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to queue task",
		})
		return
	}

	logger.Info("task submitted successfully",
		zap.String("task_id", task.ID),
		zap.String("user_id", userID),
		zap.Int("priority", int(task.Priority)),
		zap.Float64("budget", task.Budget),
	)

	// Return response
	c.JSON(http.StatusAccepted, SubmitTaskResponse{
		TaskID: task.ID,
		Status: string(task.Status),
	})
}

// GetTask retrieves a task by ID
func (h *Handlers) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	logger := h.logger.With(zap.String("handler", "GetTask"), zap.String("task_id", taskID))

	if h.taskQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service unavailable",
			"message": "task queue not available",
		})
		return
	}

	task, err := h.taskQueue.Get(taskID)
	if err != nil {
		logger.Error("failed to get task", zap.Error(err))
		if err == orchestration.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not found",
				"message": "task not found",
				"task_id": taskID,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to retrieve task",
			})
		}
		return
	}

	c.JSON(http.StatusOK, task)
}

// ListTasks lists all tasks with pagination
func (h *Handlers) ListTasks(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "ListTasks"))

	if h.taskQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service unavailable",
			"message": "task queue not available",
		})
		return
	}

	// Parse query parameters
	filter := &orchestration.TaskFilter{}

	if userID := c.Query("user_id"); userID != "" {
		filter.UserID = userID
	}

	if status := c.Query("status"); status != "" {
		filter.Status = orchestration.TaskStatus(status)
	}

	if taskType := c.Query("type"); taskType != "" {
		filter.Type = taskType
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err == nil && limit > 0 {
			filter.Limit = limit
		}
	} else {
		filter.Limit = 50 // Default limit
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	// Retrieve tasks
	tasks, err := h.taskQueue.List(filter)
	if err != nil {
		logger.Error("failed to list tasks", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "failed to retrieve tasks",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"count": len(tasks),
		"limit": filter.Limit,
		"offset": filter.Offset,
	})
}

// CancelTask cancels a running task
func (h *Handlers) CancelTask(c *gin.Context) {
	taskID := c.Param("id")
	logger := h.logger.With(zap.String("handler", "CancelTask"), zap.String("task_id", taskID))

	if h.taskQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service unavailable",
			"message": "task queue not available",
		})
		return
	}

	// Cancel the task
	err := h.taskQueue.Cancel(taskID)
	if err != nil {
		logger.Error("failed to cancel task", zap.Error(err))
		if err == orchestration.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not found",
				"message": "task not found",
				"task_id": taskID,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to cancel task",
			})
		}
		return
	}

	logger.Info("task canceled successfully", zap.String("task_id", taskID))

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"status":  "canceled",
		"message": "task canceled successfully",
	})
}

// GetTaskStatus retrieves the current status of a task
func (h *Handlers) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("id")
	logger := h.logger.With(zap.String("handler", "GetTaskStatus"), zap.String("task_id", taskID))

	if h.taskQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service unavailable",
			"message": "task queue not available",
		})
		return
	}

	task, err := h.taskQueue.Get(taskID)
	if err != nil {
		logger.Error("failed to get task", zap.Error(err))
		if err == orchestration.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not found",
				"message": "task not found",
				"task_id": taskID,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to retrieve task status",
			})
		}
		return
	}

	// Calculate progress (0-100)
	progress := 0
	switch task.Status {
	case orchestration.TaskStatusPending, orchestration.TaskStatusQueued:
		progress = 10
	case orchestration.TaskStatusAssigned:
		progress = 25
	case orchestration.TaskStatusRunning:
		progress = 50
	case orchestration.TaskStatusCompleted:
		progress = 100
	case orchestration.TaskStatusFailed, orchestration.TaskStatusCanceled:
		progress = 0
	}

	response := TaskStatusResponse{
		TaskID:     task.ID,
		Status:     string(task.Status),
		Progress:   progress,
		AssignedTo: task.AssignedTo,
		Metadata: map[string]interface{}{
			"created_at": task.CreatedAt,
			"updated_at": task.UpdatedAt,
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetTaskResult retrieves the result of a completed task
func (h *Handlers) GetTaskResult(c *gin.Context) {
	taskID := c.Param("id")
	logger := h.logger.With(zap.String("handler", "GetTaskResult"), zap.String("task_id", taskID))

	if h.taskQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service unavailable",
			"message": "task queue not available",
		})
		return
	}

	task, err := h.taskQueue.Get(taskID)
	if err != nil {
		logger.Error("failed to get task", zap.Error(err))
		if err == orchestration.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not found",
				"message": "task not found",
				"task_id": taskID,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal error",
				"message": "failed to retrieve task result",
			})
		}
		return
	}

	// Check if task is completed
	if !task.IsTerminal() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "task not completed",
			"message": "task is still in progress",
			"status":  string(task.Status),
		})
		return
	}

	// Calculate duration
	duration := int64(0)
	if task.StartedAt != nil && task.CompletedAt != nil {
		duration = task.CompletedAt.Sub(*task.StartedAt).Milliseconds()
	}

	response := TaskResultResponse{
		TaskID:   task.ID,
		Status:   string(task.Status),
		Result:   task.Result,
		Error:    task.Error,
		Cost:     task.ActualCost,
		Duration: int(duration),
		Metadata: map[string]interface{}{
			"started_at":   task.StartedAt,
			"completed_at": task.CompletedAt,
			"assigned_to":  task.AssignedTo,
		},
	}

	c.JSON(http.StatusOK, response)
}

// Helper functions

// parsePriority converts string priority to TaskPriority
func parsePriority(priorityStr string) orchestration.TaskPriority {
	switch priorityStr {
	case "critical":
		return orchestration.PriorityCritical
	case "high":
		return orchestration.PriorityHigh
	case "low":
		return orchestration.PriorityLow
	default:
		return orchestration.PriorityNormal
	}
}
