package health

import (
	"context"
	"sync/atomic"
	"time"

	ariv1 "github.com/aidenlippert/zerostate/reference-runtime-v1/pkg/ari/v1"
	"go.uber.org/zap"
)

// Service implements the ARI v1 Health service
type Service struct {
	ariv1.UnimplementedHealthServer

	logger              *zap.Logger
	startTime           time.Time
	activeTasks         int32
	totalTasksProcessed int64
}

// NewService creates a new Health service
func NewService(logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		logger:    logger,
		startTime: time.Now(),
	}
}

// Check returns the health status
func (s *Service) Check(ctx context.Context, req *ariv1.HealthCheckRequest) (*ariv1.HealthCheckResponse, error) {
	uptime := time.Since(s.startTime)
	activeTasks := atomic.LoadInt32(&s.activeTasks)
	totalProcessed := atomic.LoadInt64(&s.totalTasksProcessed)

	s.logger.Debug("Health check",
		zap.Duration("uptime", uptime),
		zap.Int32("active_tasks", activeTasks),
		zap.Int64("total_processed", totalProcessed),
	)

	return &ariv1.HealthCheckResponse{
		Status:              ariv1.HealthStatus_HEALTH_STATUS_SERVING,
		Message:             "Runtime is healthy",
		ActiveTasks:         activeTasks,
		TotalTasksProcessed: totalProcessed,
		UptimeSeconds:       int64(uptime.Seconds()),
	}, nil
}

// IncrementActiveTasks increments the active tasks counter
func (s *Service) IncrementActiveTasks() {
	atomic.AddInt32(&s.activeTasks, 1)
}

// DecrementActiveTasks decrements the active tasks counter
func (s *Service) DecrementActiveTasks() {
	atomic.AddInt32(&s.activeTasks, -1)
	atomic.AddInt64(&s.totalTasksProcessed, 1)
}
