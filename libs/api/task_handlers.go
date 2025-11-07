package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SubmitTaskRequest represents a task submission request
type SubmitTaskRequest struct {
	Query       string                 `json:"query" binding:"required"`
	Constraints map[string]interface{} `json:"constraints"`
	Budget      float64                `json:"budget" binding:"required,gt=0"`
	Timeout     int                    `json:"timeout"` // seconds
	Priority    string                 `json:"priority"` // "low", "medium", "high"
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
	// TODO: Implement task submission
	// 1. Validate request
	// 2. Queue task for orchestration
	// 3. Return task ID

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "SubmitTask endpoint not yet implemented - Sprint 7 Week 1",
	})
}

// GetTask retrieves a task by ID
func (h *Handlers) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	// TODO: Retrieve task details
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "GetTask endpoint not yet implemented",
		"task_id": taskID,
	})
}

// ListTasks lists all tasks with pagination
func (h *Handlers) ListTasks(c *gin.Context) {
	// TODO: Implement pagination and filtering
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "ListTasks endpoint not yet implemented",
	})
}

// CancelTask cancels a running task
func (h *Handlers) CancelTask(c *gin.Context) {
	taskID := c.Param("id")

	// TODO: Cancel task
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "CancelTask endpoint not yet implemented",
		"task_id": taskID,
	})
}

// GetTaskStatus retrieves the current status of a task
func (h *Handlers) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	// TODO: Get task status
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "GetTaskStatus endpoint not yet implemented",
		"task_id": taskID,
	})
}

// GetTaskResult retrieves the result of a completed task
func (h *Handlers) GetTaskResult(c *gin.Context) {
	taskID := c.Param("id")

	// TODO: Get task result
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "GetTaskResult endpoint not yet implemented",
		"task_id": taskID,
	})
}
