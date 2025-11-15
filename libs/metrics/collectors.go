package metrics

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// CustomCollector collects metrics from external sources like database
type CustomCollector struct {
	db     *sql.DB
	logger *zap.Logger

	// Descriptors for custom metrics
	agentCountDesc      *prometheus.Desc
	taskQueueDepthDesc  *prometheus.Desc
	escrowBalanceDesc   *prometheus.Desc
	reputationStatsDesc *prometheus.Desc

	// Mutex for thread safety
	mutex sync.Mutex

	// Cache for expensive queries (updated every 30s)
	lastUpdate   time.Time
	cachedValues map[string]float64
}

// NewCustomCollector creates a new custom metrics collector
func NewCustomCollector(db *sql.DB, logger *zap.Logger) *CustomCollector {
	return &CustomCollector{
		db:     db,
		logger: logger,

		agentCountDesc: prometheus.NewDesc(
			"ainur_agents_database_count",
			"Number of agents in database by status",
			[]string{"status"},
			nil,
		),

		taskQueueDepthDesc: prometheus.NewDesc(
			"ainur_task_queue_depth",
			"Current depth of task queue by priority",
			[]string{"priority"},
			nil,
		),

		escrowBalanceDesc: prometheus.NewDesc(
			"ainur_escrow_balance_total",
			"Total escrow balance by status",
			[]string{"status"},
			nil,
		),

		reputationStatsDesc: prometheus.NewDesc(
			"ainur_reputation_statistics",
			"Reputation statistics (avg, min, max)",
			[]string{"stat_type"},
			nil,
		),

		cachedValues: make(map[string]float64),
	}
}

// Describe implements the prometheus.Collector interface
func (c *CustomCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.agentCountDesc
	ch <- c.taskQueueDepthDesc
	ch <- c.escrowBalanceDesc
	ch <- c.reputationStatsDesc
}

// Collect implements the prometheus.Collector interface
func (c *CustomCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Only update cached values every 30 seconds to avoid excessive DB load
	if time.Since(c.lastUpdate) > 30*time.Second {
		c.updateCachedValues()
		c.lastUpdate = time.Now()
	}

	c.collectAgentCounts(ch)
	c.collectTaskQueueDepth(ch)
	c.collectEscrowBalance(ch)
	c.collectReputationStats(ch)
}

// updateCachedValues updates the cached metric values from database
func (c *CustomCollector) updateCachedValues() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Reset cache
	c.cachedValues = make(map[string]float64)

	// Collect agent counts
	c.updateAgentCounts(ctx)

	// Collect task queue depth
	c.updateTaskQueueDepth(ctx)

	// Collect escrow balance
	c.updateEscrowBalance(ctx)

	// Collect reputation statistics
	c.updateReputationStats(ctx)
}

func (c *CustomCollector) updateAgentCounts(ctx context.Context) {
	query := `
		SELECT
			COALESCE(status, 'unknown') as status,
			COUNT(*) as count
		FROM agents
		GROUP BY status
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		c.logger.Error("failed to query agent counts", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count float64
		if err := rows.Scan(&status, &count); err != nil {
			c.logger.Error("failed to scan agent count", zap.Error(err))
			continue
		}
		c.cachedValues["agent_count_"+status] = count
	}
}

func (c *CustomCollector) updateTaskQueueDepth(ctx context.Context) {
	query := `
		SELECT
			COALESCE(priority, 'normal') as priority,
			COUNT(*) as count
		FROM tasks
		WHERE status IN ('pending', 'running')
		GROUP BY priority
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		c.logger.Error("failed to query task queue depth", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var priority string
		var count float64
		if err := rows.Scan(&priority, &count); err != nil {
			c.logger.Error("failed to scan task queue depth", zap.Error(err))
			continue
		}
		c.cachedValues["queue_depth_"+priority] = count
	}
}

func (c *CustomCollector) updateEscrowBalance(ctx context.Context) {
	// Try the payments table first (if it exists)
	query := `
		SELECT
			status,
			SUM(CAST(amount AS DECIMAL)) as total_amount
		FROM payments
		WHERE amount IS NOT NULL
		GROUP BY status
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		// If payments table doesn't exist, provide default values
		c.logger.Warn("payments table not accessible, using defaults", zap.Error(err))
		c.cachedValues["escrow_balance_escrowed"] = 0
		c.cachedValues["escrow_balance_released"] = 0
		c.cachedValues["escrow_balance_refunded"] = 0
		return
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var amount float64
		if err := rows.Scan(&status, &amount); err != nil {
			c.logger.Error("failed to scan escrow balance", zap.Error(err))
			continue
		}
		c.cachedValues["escrow_balance_"+status] = amount
	}
}

func (c *CustomCollector) updateReputationStats(ctx context.Context) {
	// Try to get reputation statistics
	query := `
		SELECT
			AVG(CAST(reputation_score AS DECIMAL)) as avg_reputation,
			MIN(CAST(reputation_score AS DECIMAL)) as min_reputation,
			MAX(CAST(reputation_score AS DECIMAL)) as max_reputation,
			COUNT(*) as agent_count
		FROM agents
		WHERE reputation_score IS NOT NULL
	`

	var avgRep, minRep, maxRep, agentCount sql.NullFloat64
	err := c.db.QueryRowContext(ctx, query).Scan(&avgRep, &minRep, &maxRep, &agentCount)
	if err != nil {
		c.logger.Warn("failed to query reputation stats", zap.Error(err))
		// Set defaults
		c.cachedValues["reputation_avg"] = 0
		c.cachedValues["reputation_min"] = 0
		c.cachedValues["reputation_max"] = 0
		return
	}

	if avgRep.Valid {
		c.cachedValues["reputation_avg"] = avgRep.Float64
	}
	if minRep.Valid {
		c.cachedValues["reputation_min"] = minRep.Float64
	}
	if maxRep.Valid {
		c.cachedValues["reputation_max"] = maxRep.Float64
	}
}

func (c *CustomCollector) collectAgentCounts(ch chan<- prometheus.Metric) {
	statuses := []string{"active", "inactive", "pending", "banned", "unknown"}

	for _, status := range statuses {
		key := "agent_count_" + status
		count := c.cachedValues[key]

		ch <- prometheus.MustNewConstMetric(
			c.agentCountDesc,
			prometheus.GaugeValue,
			count,
			status,
		)
	}
}

func (c *CustomCollector) collectTaskQueueDepth(ch chan<- prometheus.Metric) {
	priorities := []string{"high", "normal", "low"}

	for _, priority := range priorities {
		key := "queue_depth_" + priority
		depth := c.cachedValues[key]

		ch <- prometheus.MustNewConstMetric(
			c.taskQueueDepthDesc,
			prometheus.GaugeValue,
			depth,
			priority,
		)
	}
}

func (c *CustomCollector) collectEscrowBalance(ch chan<- prometheus.Metric) {
	statuses := []string{"escrowed", "released", "refunded", "disputed"}

	for _, status := range statuses {
		key := "escrow_balance_" + status
		balance := c.cachedValues[key]

		ch <- prometheus.MustNewConstMetric(
			c.escrowBalanceDesc,
			prometheus.GaugeValue,
			balance,
			status,
		)
	}
}

func (c *CustomCollector) collectReputationStats(ch chan<- prometheus.Metric) {
	stats := []string{"avg", "min", "max"}

	for _, stat := range stats {
		key := "reputation_" + stat
		value := c.cachedValues[key]

		ch <- prometheus.MustNewConstMetric(
			c.reputationStatsDesc,
			prometheus.GaugeValue,
			value,
			stat,
		)
	}
}

// HealthCollector provides health-related metrics
type HealthCollector struct {
	uptime           time.Time
	uptimeDesc       *prometheus.Desc
	healthCheckDesc  *prometheus.Desc
	serviceStatusDesc *prometheus.Desc

	// Health check functions
	healthChecks map[string]func() bool
	mutex        sync.RWMutex
}

// NewHealthCollector creates a new health metrics collector
func NewHealthCollector() *HealthCollector {
	return &HealthCollector{
		uptime: time.Now(),

		uptimeDesc: prometheus.NewDesc(
			"ainur_uptime_seconds",
			"Service uptime in seconds",
			nil,
			nil,
		),

		healthCheckDesc: prometheus.NewDesc(
			"ainur_health_check_status",
			"Health check status (1=healthy, 0=unhealthy)",
			[]string{"check_name"},
			nil,
		),

		serviceStatusDesc: prometheus.NewDesc(
			"ainur_service_status",
			"Service component status (1=up, 0=down)",
			[]string{"service"},
			nil,
		),

		healthChecks: make(map[string]func() bool),
	}
}

// RegisterHealthCheck registers a health check function
func (h *HealthCollector) RegisterHealthCheck(name string, checkFunc func() bool) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.healthChecks[name] = checkFunc
}

// Describe implements the prometheus.Collector interface
func (h *HealthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- h.uptimeDesc
	ch <- h.healthCheckDesc
	ch <- h.serviceStatusDesc
}

// Collect implements the prometheus.Collector interface
func (h *HealthCollector) Collect(ch chan<- prometheus.Metric) {
	// Collect uptime
	uptime := time.Since(h.uptime).Seconds()
	ch <- prometheus.MustNewConstMetric(
		h.uptimeDesc,
		prometheus.GaugeValue,
		uptime,
	)

	// Collect health checks
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for name, checkFunc := range h.healthChecks {
		status := 0.0
		if checkFunc() {
			status = 1.0
		}

		ch <- prometheus.MustNewConstMetric(
			h.healthCheckDesc,
			prometheus.GaugeValue,
			status,
			name,
		)
	}

	// Collect service statuses (these would be updated by the main application)
	services := []string{"database", "blockchain", "p2p", "orchestrator", "api"}
	for _, service := range services {
		// Default to healthy - this would be updated by the actual service monitoring
		ch <- prometheus.MustNewConstMetric(
			h.serviceStatusDesc,
			prometheus.GaugeValue,
			1.0, // Default healthy
			service,
		)
	}
}

// MetricsCollectorManager manages all custom collectors
type MetricsCollectorManager struct {
	registry        *prometheus.Registry
	customCollector *CustomCollector
	healthCollector *HealthCollector
	logger          *zap.Logger
}

// NewMetricsCollectorManager creates a new collector manager
func NewMetricsCollectorManager(db *sql.DB, logger *zap.Logger) *MetricsCollectorManager {
	registry := prometheus.NewRegistry()

	customCollector := NewCustomCollector(db, logger)
	healthCollector := NewHealthCollector()

	// Register collectors
	registry.MustRegister(customCollector)
	registry.MustRegister(healthCollector)

	// Register standard Go metrics
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return &MetricsCollectorManager{
		registry:        registry,
		customCollector: customCollector,
		healthCollector: healthCollector,
		logger:          logger,
	}
}

// GetRegistry returns the Prometheus registry
func (m *MetricsCollectorManager) GetRegistry() *prometheus.Registry {
	return m.registry
}

// RegisterHealthCheck registers a health check with the health collector
func (m *MetricsCollectorManager) RegisterHealthCheck(name string, checkFunc func() bool) {
	m.healthCollector.RegisterHealthCheck(name, checkFunc)
}

// GetHealthSummary returns a summary of all health checks
func (m *MetricsCollectorManager) GetHealthSummary() map[string]interface{} {
	summary := map[string]interface{}{
		"uptime_seconds": time.Since(m.healthCollector.uptime).Seconds(),
		"health_checks":  make(map[string]bool),
		"services":       make(map[string]bool),
	}

	m.healthCollector.mutex.RLock()
	defer m.healthCollector.mutex.RUnlock()

	healthChecks := summary["health_checks"].(map[string]bool)
	for name, checkFunc := range m.healthCollector.healthChecks {
		healthChecks[name] = checkFunc()
	}

	// Default service statuses
	services := summary["services"].(map[string]bool)
	defaultServices := []string{"database", "blockchain", "p2p", "orchestrator", "api"}
	for _, service := range defaultServices {
		services[service] = true // Default to healthy
	}

	return summary
}