package api

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status      string                 `json:"status"`
	Timestamp   string                 `json:"timestamp"`
	Version     string                 `json:"version"`
	Uptime      float64                `json:"uptime_seconds"`
	Services    map[string]ServiceHealth `json:"services"`
	Metrics     MetricsSummary         `json:"metrics"`
	Runtime     RuntimeInfo            `json:"runtime"`
	SLO         SLOMetrics             `json:"slo"`
}

// ServiceHealth represents the health of a service component
type ServiceHealth struct {
	Status      string    `json:"status"`
	LastCheck   string    `json:"last_check"`
	Message     string    `json:"message,omitempty"`
	ResponseTime float64  `json:"response_time_ms,omitempty"`
}

// MetricsSummary provides a summary of key metrics
type MetricsSummary struct {
	TasksProcessed     int64   `json:"tasks_processed_total"`
	TasksInQueue       int     `json:"tasks_in_queue"`
	AgentsRegistered   int     `json:"agents_registered"`
	ActiveConnections  int     `json:"active_connections"`
	ErrorRate          float64 `json:"error_rate_percent"`
	P95Latency         float64 `json:"p95_latency_ms"`
	ThroughputRPS      float64 `json:"throughput_rps"`
}

// RuntimeInfo provides Go runtime information
type RuntimeInfo struct {
	GoVersion      string  `json:"go_version"`
	Goroutines     int     `json:"goroutines"`
	MemoryAllocMB  float64 `json:"memory_alloc_mb"`
	MemoryTotalMB  float64 `json:"memory_total_mb"`
	MemorySystemMB float64 `json:"memory_system_mb"`
	GCCycles       uint32  `json:"gc_cycles"`
	GCPauseMs      float64 `json:"gc_pause_ms"`
}

// SLOMetrics provides Service Level Objective metrics
type SLOMetrics struct {
	UptimeTarget      float64 `json:"uptime_target_percent"`
	LatencyTarget     float64 `json:"latency_target_ms"`
	ErrorRateTarget   float64 `json:"error_rate_target_percent"`
	CurrentUptime     float64 `json:"current_uptime_percent"`
	CurrentLatency    float64 `json:"current_latency_ms"`
	CurrentErrorRate  float64 `json:"current_error_rate_percent"`
	UptimeSLOStatus   string  `json:"uptime_slo_status"`
	LatencySLOStatus  string  `json:"latency_slo_status"`
	ErrorSLOStatus    string  `json:"error_slo_status"`
}

// EnhancedHealthHandler provides comprehensive health check functionality
type EnhancedHealthHandler struct {
	handlers        *Handlers
	metricsHandler  *MetricsHandler
	logger          *zap.Logger
	startTime       time.Time
	version         string

	// Health check functions
	healthCheckers  map[string]func() ServiceHealth
}

// NewEnhancedHealthHandler creates a new enhanced health handler
func NewEnhancedHealthHandler(handlers *Handlers, metricsHandler *MetricsHandler, logger *zap.Logger) *EnhancedHealthHandler {
	eh := &EnhancedHealthHandler{
		handlers:       handlers,
		metricsHandler: metricsHandler,
		logger:         logger,
		startTime:      time.Now(),
		version:        "v1.0.0", // Could be injected from build
		healthCheckers: make(map[string]func() ServiceHealth),
	}

	// Register default health checkers
	eh.registerDefaultHealthCheckers()

	return eh
}

// registerDefaultHealthCheckers registers the default health check functions
func (eh *EnhancedHealthHandler) registerDefaultHealthCheckers() {
	// Database health checker
	eh.healthCheckers["database"] = func() ServiceHealth {
		start := time.Now()
		status := "healthy"
		message := "Database connection is healthy"

		if eh.handlers.db == nil {
			return ServiceHealth{
				Status:      "down",
				LastCheck:   time.Now().UTC().Format(time.RFC3339),
				Message:     "Database not initialized",
				ResponseTime: 0,
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := eh.handlers.db.Conn().PingContext(ctx); err != nil {
			status = "unhealthy"
			message = "Database ping failed: " + err.Error()
		}

		return ServiceHealth{
			Status:      status,
			LastCheck:   time.Now().UTC().Format(time.RFC3339),
			Message:     message,
			ResponseTime: float64(time.Since(start).Nanoseconds()) / 1e6,
		}
	}

	// Blockchain health checker
	eh.healthCheckers["blockchain"] = func() ServiceHealth {
		start := time.Now()
		status := "healthy"
		message := "Blockchain connection is healthy"

		if eh.handlers.blockchain == nil {
			return ServiceHealth{
				Status:      "down",
				LastCheck:   time.Now().UTC().Format(time.RFC3339),
				Message:     "Blockchain service not initialized",
				ResponseTime: 0,
			}
		}

		if !eh.handlers.blockchain.IsEnabled() {
			status = "unhealthy"
			message = "Blockchain not connected"
		}

		return ServiceHealth{
			Status:      status,
			LastCheck:   time.Now().UTC().Format(time.RFC3339),
			Message:     message,
			ResponseTime: float64(time.Since(start).Nanoseconds()) / 1e6,
		}
	}

	// Orchestrator health checker
	eh.healthCheckers["orchestrator"] = func() ServiceHealth {
		start := time.Now()
		status := "healthy"
		message := "Orchestrator is running"

		if eh.handlers.orchestrator == nil {
			status = "down"
			message = "Orchestrator not initialized"
		}

		return ServiceHealth{
			Status:      status,
			LastCheck:   time.Now().UTC().Format(time.RFC3339),
			Message:     message,
			ResponseTime: float64(time.Since(start).Nanoseconds()) / 1e6,
		}
	}

	// P2P health checker
	eh.healthCheckers["p2p"] = func() ServiceHealth {
		start := time.Now()
		status := "healthy"
		message := "P2P node is running"

		if eh.handlers.p2pHost == nil {
			status = "down"
			message = "P2P host not initialized"
		} else {
			// Check if we have any connections
			connections := len(eh.handlers.p2pHost.Network().Peers())
			message = fmt.Sprintf("P2P node running with %d peers", connections)
		}

		return ServiceHealth{
			Status:      status,
			LastCheck:   time.Now().UTC().Format(time.RFC3339),
			Message:     message,
			ResponseTime: float64(time.Since(start).Nanoseconds()) / 1e6,
		}
	}

	// Task Queue health checker
	eh.healthCheckers["task_queue"] = func() ServiceHealth {
		start := time.Now()
		status := "healthy"
		message := "Task queue is operational"

		if eh.handlers.taskQueue == nil {
			status = "down"
			message = "Task queue not initialized"
		}

		return ServiceHealth{
			Status:      status,
			LastCheck:   time.Now().UTC().Format(time.RFC3339),
			Message:     message,
			ResponseTime: float64(time.Since(start).Nanoseconds()) / 1e6,
		}
	}
}

// HandleHealth provides basic health check
func (eh *EnhancedHealthHandler) HandleHealth(c *gin.Context) {
	logger := eh.logger.With(
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
	)

	logger.Info("serving basic health check")

	// Perform quick health check
	overallStatus := eh.getOverallHealthStatus()

	response := gin.H{
		"status":    overallStatus,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(eh.startTime).Seconds(),
		"version":   eh.version,
	}

	statusCode := http.StatusOK
	if overallStatus != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// HandleDetailedHealth provides comprehensive health information
func (eh *EnhancedHealthHandler) HandleDetailedHealth(c *gin.Context) {
	logger := eh.logger.With(
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
	)

	logger.Info("serving detailed health check")

	// Gather all health information
	services := make(map[string]ServiceHealth)
	for name, checker := range eh.healthCheckers {
		services[name] = checker()
	}

	// Get metrics summary
	metricsSummary := eh.getMetricsSummary()

	// Get runtime info
	runtimeInfo := eh.getRuntimeInfo()

	// Get SLO metrics
	sloMetrics := eh.getSLOMetrics()

	// Determine overall status
	overallStatus := "healthy"
	for _, service := range services {
		if service.Status == "down" || service.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		}
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   eh.version,
		Uptime:    time.Since(eh.startTime).Seconds(),
		Services:  services,
		Metrics:   metricsSummary,
		Runtime:   runtimeInfo,
		SLO:       sloMetrics,
	}

	statusCode := http.StatusOK
	if overallStatus != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	// Add custom headers for monitoring systems
	c.Header("X-Health-Status", overallStatus)
	c.Header("X-Service-Version", eh.version)
	c.Header("X-Uptime-Seconds", fmt.Sprintf("%.0f", response.Uptime))

	c.JSON(statusCode, response)
}

// HandleReadiness provides readiness check for Kubernetes
func (eh *EnhancedHealthHandler) HandleReadiness(c *gin.Context) {
	logger := eh.logger.With(
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
	)

	logger.Info("serving readiness check")

	// Check critical services for readiness
	criticalServices := []string{"database", "orchestrator"}
	ready := true
	failedServices := []string{}

	for _, serviceName := range criticalServices {
		if checker, exists := eh.healthCheckers[serviceName]; exists {
			health := checker()
			if health.Status == "down" || health.Status == "unhealthy" {
				ready = false
				failedServices = append(failedServices, serviceName)
			}
		}
	}

	response := gin.H{
		"ready":     ready,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   eh.version,
	}

	if !ready {
		response["failed_services"] = failedServices
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// Helper methods

func (eh *EnhancedHealthHandler) getOverallHealthStatus() string {
	for _, checker := range eh.healthCheckers {
		health := checker()
		if health.Status == "down" || health.Status == "unhealthy" {
			return "unhealthy"
		}
	}
	return "healthy"
}

func (eh *EnhancedHealthHandler) getMetricsSummary() MetricsSummary {
	// In a real implementation, these would be queried from the metrics system
	return MetricsSummary{
		TasksProcessed:    1250,
		TasksInQueue:      5,
		AgentsRegistered:  25,
		ActiveConnections: 12,
		ErrorRate:         0.5,
		P95Latency:        45.2,
		ThroughputRPS:     15.3,
	}
}

func (eh *EnhancedHealthHandler) getRuntimeInfo() RuntimeInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return RuntimeInfo{
		GoVersion:      runtime.Version(),
		Goroutines:     runtime.NumGoroutine(),
		MemoryAllocMB:  float64(memStats.Alloc) / 1024 / 1024,
		MemoryTotalMB:  float64(memStats.TotalAlloc) / 1024 / 1024,
		MemorySystemMB: float64(memStats.Sys) / 1024 / 1024,
		GCCycles:       memStats.NumGC,
		GCPauseMs:      float64(memStats.PauseTotalNs) / 1e6,
	}
}

func (eh *EnhancedHealthHandler) getSLOMetrics() SLOMetrics {
	// SLO targets
	uptimeTarget := 99.9
	latencyTarget := 100.0
	errorRateTarget := 1.0

	// Calculate current metrics (placeholders - would be real calculations)
	currentUptime := 99.95
	currentLatency := 45.2
	currentErrorRate := 0.5

	return SLOMetrics{
		UptimeTarget:     uptimeTarget,
		LatencyTarget:    latencyTarget,
		ErrorRateTarget:  errorRateTarget,
		CurrentUptime:    currentUptime,
		CurrentLatency:   currentLatency,
		CurrentErrorRate: currentErrorRate,
		UptimeSLOStatus:  eh.getSLOStatus(currentUptime, uptimeTarget, true),
		LatencySLOStatus: eh.getSLOStatus(currentLatency, latencyTarget, false),
		ErrorSLOStatus:   eh.getSLOStatus(currentErrorRate, errorRateTarget, false),
	}
}

func (eh *EnhancedHealthHandler) getSLOStatus(current, target float64, higherIsBetter bool) string {
	if higherIsBetter {
		if current >= target {
			return "meeting"
		}
	} else {
		if current <= target {
			return "meeting"
		}
	}
	return "failing"
}

// RegisterHealthChecker allows registering custom health checkers
func (eh *EnhancedHealthHandler) RegisterHealthChecker(name string, checker func() ServiceHealth) {
	eh.healthCheckers[name] = checker
}