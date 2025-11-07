package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OrchestratorMetricsResponse represents orchestrator metrics
type OrchestratorMetricsResponse struct {
	TasksProcessed   int64   `json:"tasks_processed"`
	TasksSucceeded   int64   `json:"tasks_succeeded"`
	TasksFailed      int64   `json:"tasks_failed"`
	TasksTimedOut    int64   `json:"tasks_timed_out"`
	AvgExecutionMS   int64   `json:"avg_execution_ms"`
	ActiveWorkers    int     `json:"active_workers"`
	SuccessRate      float64 `json:"success_rate"`
}

// GetOrchestratorMetrics retrieves orchestrator performance metrics
func (h *Handlers) GetOrchestratorMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetOrchestratorMetrics"))

	if h.orchestrator == nil {
		logger.Warn("orchestrator not initialized")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service unavailable",
			"message": "orchestrator not available",
		})
		return
	}

	// Get metrics from orchestrator
	metrics := h.orchestrator.GetMetrics()

	// Calculate success rate
	successRate := 0.0
	if metrics.TasksProcessed > 0 {
		successRate = float64(metrics.TasksSucceeded) / float64(metrics.TasksProcessed) * 100
	}

	response := OrchestratorMetricsResponse{
		TasksProcessed: metrics.TasksProcessed,
		TasksSucceeded: metrics.TasksSucceeded,
		TasksFailed:    metrics.TasksFailed,
		TasksTimedOut:  metrics.TasksTimedOut,
		AvgExecutionMS: metrics.AvgExecutionTime.Milliseconds(),
		ActiveWorkers:  metrics.ActiveWorkers,
		SuccessRate:    successRate,
	}

	c.JSON(http.StatusOK, response)
}

// GetOrchestratorHealth checks orchestrator health status
func (h *Handlers) GetOrchestratorHealth(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetOrchestratorHealth"))

	if h.orchestrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"reason": "orchestrator not initialized",
		})
		return
	}

	metrics := h.orchestrator.GetMetrics()

	// Simple health check based on success rate
	status := "healthy"
	if metrics.TasksProcessed > 10 {
		successRate := float64(metrics.TasksSucceeded) / float64(metrics.TasksProcessed)
		if successRate < 0.5 {
			status = "degraded"
			logger.Warn("orchestrator health degraded",
				zap.Float64("success_rate", successRate),
			)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          status,
		"tasks_processed": metrics.TasksProcessed,
		"success_rate":    float64(metrics.TasksSucceeded) / float64(metrics.TasksProcessed),
		"active_workers":  metrics.ActiveWorkers,
	})
}
