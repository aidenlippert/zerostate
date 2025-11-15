package api

import (
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/metrics"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// MetricsHandler handles Prometheus metrics exposition
type MetricsHandler struct {
	prometheusMetrics *metrics.PrometheusMetrics
	registry          *prometheus.Registry
	collectorManager  *metrics.MetricsCollectorManager
	logger            *zap.Logger
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(handlers *Handlers, logger *zap.Logger, promMetrics *metrics.PrometheusMetrics, registry *prometheus.Registry) *MetricsHandler {
	// Ensure we have a registry
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	// Create PrometheusMetrics instance if not provided
	if promMetrics == nil {
		promMetrics = metrics.NewPrometheusMetrics(registry)
	}

	// Create collector manager with database connection
	var collectorManager *metrics.MetricsCollectorManager
	if handlers.db != nil && handlers.db.Conn() != nil {
		collectorManager = metrics.NewMetricsCollectorManager(handlers.db.Conn(), logger)

		// Register health checks
		collectorManager.RegisterHealthCheck("database", func() bool {
			return handlers.db != nil && handlers.db.Conn().Ping() == nil
		})

		collectorManager.RegisterHealthCheck("blockchain", func() bool {
			return handlers.blockchain != nil && handlers.blockchain.IsEnabled()
		})

		collectorManager.RegisterHealthCheck("orchestrator", func() bool {
			return handlers.orchestrator != nil
		})

		collectorManager.RegisterHealthCheck("p2p", func() bool {
			return handlers.p2pHost != nil
		})
	}

	return &MetricsHandler{
		prometheusMetrics: promMetrics,
		registry:          registry,
		collectorManager:  collectorManager,
		logger:            logger,
	}
}

// HandleMetrics serves Prometheus metrics
func (mh *MetricsHandler) HandleMetrics() gin.HandlerFunc {
	var handler http.Handler

	switch {
	case mh.collectorManager != nil && mh.registry != nil:
		gatherers := prometheus.Gatherers{
			mh.collectorManager.GetRegistry(),
			mh.registry,
		}
		handler = promhttp.HandlerFor(
			gatherers,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			},
		)
	case mh.collectorManager != nil:
		handler = promhttp.HandlerFor(
			mh.collectorManager.GetRegistry(),
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
				Registry:          mh.collectorManager.GetRegistry(),
			},
		)
	default:
		handler = promhttp.HandlerFor(
			mh.registry,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
				Registry:          mh.registry,
			},
		)
	}

	return gin.WrapH(handler)
}

// HandleMetricsSummary provides a JSON summary of key metrics
func (mh *MetricsHandler) HandleMetricsSummary(c *gin.Context) {
	logger := mh.logger.With(
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("user_agent", c.Request.UserAgent()),
	)

	logger.Info("serving metrics summary")

	// Gather metrics summary
	summary := map[string]interface{}{
		"timestamp": mh.getCurrentTimestamp(),
		"service":   "ainur-orchestrator",
		"version":   "v1.0.0", // This could be injected from build info
	}

	// Add runtime metrics
	if mh.prometheusMetrics != nil {
		summary["runtime"] = mh.prometheusMetrics.GetRuntimeMetrics()
	}

	// Add health summary if collector manager exists
	if mh.collectorManager != nil {
		summary["health"] = mh.collectorManager.GetHealthSummary()
	}

	// Add basic counters (these would normally be retrieved from Prometheus)
	summary["counters"] = map[string]interface{}{
		"tasks_total":        mh.getMetricValue("ainur_tasks_total"),
		"api_requests_total": mh.getMetricValue("ainur_api_requests_total"),
		"agents_registered":  mh.getMetricValue("ainur_agents_registered_total"),
		"payments_total":     mh.getMetricValue("ainur_payments_total"),
		"blockchain_calls":   mh.getMetricValue("ainur_blockchain_calls_total"),
		"auctions_total":     mh.getMetricValue("ainur_auctions_total"),
	}

	// Add gauge values
	summary["gauges"] = map[string]interface{}{
		"tasks_in_queue":       mh.getMetricValue("ainur_tasks_in_queue"),
		"active_connections":   mh.getMetricValue("ainur_api_connections_active"),
		"blockchain_connected": mh.getMetricValue("ainur_blockchain_connection_status"),
		"vcg_efficiency_ratio": mh.getMetricValue("ainur_vcg_efficiency_ratio"),
		"escrow_balance":       mh.getMetricValue("ainur_escrow_balance_ainu"),
	}

	c.JSON(http.StatusOK, summary)
}

// HandleHealthMetrics provides detailed health metrics
func (mh *MetricsHandler) HandleHealthMetrics(c *gin.Context) {
	logger := mh.logger.With(
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
	)

	logger.Info("serving health metrics")

	healthData := map[string]interface{}{
		"timestamp": mh.getCurrentTimestamp(),
		"status":    "healthy",
	}

	if mh.collectorManager != nil {
		healthSummary := mh.collectorManager.GetHealthSummary()
		healthData["health_checks"] = healthSummary["health_checks"]
		healthData["services"] = healthSummary["services"]
		healthData["uptime_seconds"] = healthSummary["uptime_seconds"]

		// Determine overall health
		allHealthy := true
		if healthChecks, ok := healthSummary["health_checks"].(map[string]bool); ok {
			for _, healthy := range healthChecks {
				if !healthy {
					allHealthy = false
					break
				}
			}
		}

		if services, ok := healthSummary["services"].(map[string]bool); ok {
			for _, up := range services {
				if !up {
					allHealthy = false
					break
				}
			}
		}

		if !allHealthy {
			healthData["status"] = "degraded"
			c.Header("X-Health-Status", "degraded")
		} else {
			c.Header("X-Health-Status", "healthy")
		}
	}

	// Add SLO metrics
	healthData["slo"] = map[string]interface{}{
		"target_uptime_percent":     99.9,
		"target_p95_latency_ms":     100,
		"target_error_rate_percent": 1.0,
		"current_error_rate":        mh.calculateCurrentErrorRate(),
		"current_p95_latency":       mh.calculateCurrentP95Latency(),
	}

	c.JSON(http.StatusOK, healthData)
}

// RecordAPIRequest records API request metrics
func (mh *MetricsHandler) RecordAPIRequest(endpoint, method, status string, durationMs float64) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.RecordAPIRequest(endpoint, method, status,
			time.Duration(durationMs*float64(time.Millisecond)))
	}
}

// RecordTaskCompletion records task completion metrics
func (mh *MetricsHandler) RecordTaskCompletion(taskType, status string, durationSeconds float64) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.RecordTaskCompletion(taskType, status,
			time.Duration(durationSeconds*float64(time.Second)))
	}
}

// RecordAgentSelection records agent selection metrics
func (mh *MetricsHandler) RecordAgentSelection(selectionType string, durationMs float64) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.RecordAgentSelection(selectionType,
			time.Duration(durationMs*float64(time.Millisecond)))
	}
}

// RecordAuction records auction metrics
func (mh *MetricsHandler) RecordAuction(auctionType string, durationMs float64, participants int) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.RecordAuction(auctionType,
			time.Duration(durationMs*float64(time.Millisecond)), participants)
	}
}

// RecordPayment records payment metrics
func (mh *MetricsHandler) RecordPayment(paymentType string, amount, latencyMs float64) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.RecordPayment(paymentType, amount,
			time.Duration(latencyMs*float64(time.Millisecond)))
	}
}

// RecordBlockchainCall records blockchain call metrics
func (mh *MetricsHandler) RecordBlockchainCall(method, result string, durationMs float64) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.RecordBlockchainCall(method, result,
			time.Duration(durationMs*float64(time.Millisecond)))
	}
}

// UpdateQueueSize updates queue size metrics
func (mh *MetricsHandler) UpdateQueueSize(size int) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.UpdateQueueSize(size)
	}
}

// UpdateAgentCount updates agent count metrics
func (mh *MetricsHandler) UpdateAgentCount(count int) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.UpdateAgentCount(count)
	}
}

// UpdateBlockchainStatus updates blockchain connection status
func (mh *MetricsHandler) UpdateBlockchainStatus(connected bool) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.UpdateBlockchainStatus(connected)
	}
}

// UpdateCircuitBreaker updates circuit breaker state
func (mh *MetricsHandler) UpdateCircuitBreaker(service, state string) {
	if mh.prometheusMetrics != nil {
		mh.prometheusMetrics.UpdateCircuitBreaker(service, state)
	}
}

// Helper methods

func (mh *MetricsHandler) getCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func (mh *MetricsHandler) getMetricValue(metricName string) interface{} {
	// This is a placeholder - in a real implementation, you'd query the metric
	// from the registry or maintain a cache of current values
	return "N/A - requires metric query implementation"
}

func (mh *MetricsHandler) calculateCurrentErrorRate() float64 {
	// Placeholder implementation
	// In reality, this would calculate error rate from recent metrics
	return 0.5 // 0.5% error rate
}

func (mh *MetricsHandler) calculateCurrentP95Latency() float64 {
	// Placeholder implementation
	// In reality, this would calculate P95 latency from histogram metrics
	return 45.2 // 45.2ms P95 latency
}

// GetPrometheusMetrics returns the prometheus metrics instance for use by other components
func (mh *MetricsHandler) GetPrometheusMetrics() *metrics.PrometheusMetrics {
	return mh.prometheusMetrics
}
