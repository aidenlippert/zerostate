package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	providerRefreshes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_provider_refreshes_total",
			Help: "Total number of provider record refreshes",
		},
		[]string{"cid", "result"}, // success, failure
	)

	providerRefreshLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zerostate_provider_refresh_latency_seconds",
			Help:    "Latency of provider record refresh operations",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
		},
		[]string{"cid"},
	)

	activeProviderRecords = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zerostate_active_provider_records",
			Help: "Number of active provider records being refreshed",
		},
	)

	providerTTLRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zerostate_provider_ttl_remaining_seconds",
			Help: "Time remaining until provider record expires",
		},
		[]string{"cid"},
	)
)

const (
	// DefaultProviderTTL is the default TTL for provider records (24 hours)
	DefaultProviderTTL = 24 * time.Hour
	// DefaultRefreshInterval is how often to refresh (12 hours - 50% of TTL)
	DefaultRefreshInterval = 12 * time.Hour
	// MinRefreshInterval is the minimum refresh interval
	MinRefreshInterval = 1 * time.Hour
)

// ProviderRecord tracks a content provider advertisement
type ProviderRecord struct {
	CID             cid.Cid
	ProvidedAt      time.Time
	LastRefresh     time.Time
	NextRefresh     time.Time
	RefreshInterval time.Duration
	RefreshCount    int
	Metadata        map[string]interface{}
}

// ProviderRefresher manages automatic provider record refreshing
type ProviderRefresher struct {
	mu       sync.RWMutex
	dht      *dht.IpfsDHT
	records  map[string]*ProviderRecord // CID string -> record
	logger   *zap.Logger
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
	ticker   *time.Ticker
}

// ProviderRefresherConfig holds configuration
type ProviderRefresherConfig struct {
	RefreshInterval time.Duration
	EnableMetrics   bool
}

// DefaultProviderRefresherConfig returns default configuration
func DefaultProviderRefresherConfig() *ProviderRefresherConfig {
	return &ProviderRefresherConfig{
		RefreshInterval: DefaultRefreshInterval,
		EnableMetrics:   true,
	}
}

// NewProviderRefresher creates a new provider refresher
func NewProviderRefresher(ctx context.Context, dht *dht.IpfsDHT, config *ProviderRefresherConfig, logger *zap.Logger) *ProviderRefresher {
	if config == nil {
		config = DefaultProviderRefresherConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Ensure minimum interval
	if config.RefreshInterval < MinRefreshInterval {
		config.RefreshInterval = MinRefreshInterval
		logger.Warn("refresh interval too small, using minimum",
			zap.Duration("min_interval", MinRefreshInterval),
		)
	}

	refreshCtx, cancel := context.WithCancel(ctx)

	pr := &ProviderRefresher{
		dht:      dht,
		records:  make(map[string]*ProviderRecord),
		logger:   logger,
		ctx:      refreshCtx,
		cancel:   cancel,
		interval: config.RefreshInterval,
		ticker:   time.NewTicker(config.RefreshInterval),
	}

	// Start refresh loop
	go pr.refreshLoop()

	logger.Info("provider refresher started",
		zap.Duration("interval", config.RefreshInterval),
	)

	return pr
}

// Provide adds a CID to be provided and starts refreshing it
func (pr *ProviderRefresher) Provide(ctx context.Context, c cid.Cid) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	cidStr := c.String()

	// Check if already providing
	if record, exists := pr.records[cidStr]; exists {
		pr.logger.Debug("already providing CID",
			zap.String("cid", cidStr),
			zap.Int("refresh_count", record.RefreshCount),
		)
		return nil
	}

	// Provide to DHT (may fail if no peers, but we still track it)
	start := time.Now()
	err := pr.dht.Provide(ctx, c, true)
	latency := time.Since(start).Seconds()
	
	// Create record even if provide failed (for testing/offline scenarios)
	now := time.Now()
	record := &ProviderRecord{
		CID:             c,
		ProvidedAt:      now,
		LastRefresh:     now,
		NextRefresh:     now.Add(pr.interval),
		RefreshInterval: pr.interval,
		RefreshCount:    0,
		Metadata:        make(map[string]interface{}),
	}

	pr.records[cidStr] = record
	activeProviderRecords.Set(float64(len(pr.records)))

	if err != nil {
		providerRefreshes.WithLabelValues(cidStr, "failure").Inc()
		pr.logger.Warn("failed to provide CID to DHT (will retry on refresh)",
			zap.String("cid", cidStr),
			zap.Error(err),
		)
	} else {
		providerRefreshLatency.WithLabelValues(cidStr).Observe(latency)
		providerRefreshes.WithLabelValues(cidStr, "success").Inc()
		pr.logger.Info("providing CID",
			zap.String("cid", cidStr),
			zap.Duration("refresh_interval", pr.interval),
		)
	}

	return nil
}

// Unprovide stops providing a CID
func (pr *ProviderRefresher) Unprovide(c cid.Cid) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	cidStr := c.String()
	delete(pr.records, cidStr)
	activeProviderRecords.Set(float64(len(pr.records)))

	pr.logger.Info("stopped providing CID", zap.String("cid", cidStr))
}

// GetRecord returns a provider record
func (pr *ProviderRefresher) GetRecord(c cid.Cid) (*ProviderRecord, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	record, exists := pr.records[c.String()]
	return record, exists
}

// ListRecords returns all active provider records
func (pr *ProviderRefresher) ListRecords() []*ProviderRecord {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	records := make([]*ProviderRecord, 0, len(pr.records))
	for _, record := range pr.records {
		records = append(records, record)
	}
	return records
}

// refreshLoop periodically refreshes provider records
func (pr *ProviderRefresher) refreshLoop() {
	for {
		select {
		case <-pr.ticker.C:
			pr.refreshAll()
		case <-pr.ctx.Done():
			return
		}
	}
}

// refreshAll refreshes all provider records that are due
func (pr *ProviderRefresher) refreshAll() {
	pr.mu.RLock()
	toRefresh := make([]*ProviderRecord, 0)
	now := time.Now()

	for _, record := range pr.records {
		if now.After(record.NextRefresh) || now.Equal(record.NextRefresh) {
			toRefresh = append(toRefresh, record)
		}
		
		// Update TTL metric
		ttlRemaining := record.NextRefresh.Sub(now).Seconds()
		if ttlRemaining < 0 {
			ttlRemaining = 0
		}
		providerTTLRemaining.WithLabelValues(record.CID.String()).Set(ttlRemaining)
	}
	pr.mu.RUnlock()

	// Refresh outside of lock
	for _, record := range toRefresh {
		if err := pr.refresh(record); err != nil {
			pr.logger.Error("failed to refresh provider record",
				zap.String("cid", record.CID.String()),
				zap.Error(err),
			)
		}
	}

	if len(toRefresh) > 0 {
		pr.logger.Info("refreshed provider records",
			zap.Int("count", len(toRefresh)),
		)
	}
}

// refresh refreshes a single provider record
func (pr *ProviderRefresher) refresh(record *ProviderRecord) error {
	cidStr := record.CID.String()

	// Provide to DHT with timeout
	ctx, cancel := context.WithTimeout(pr.ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	err := pr.dht.Provide(ctx, record.CID, true)
	latency := time.Since(start).Seconds()

	// Update record even if provide failed (will retry next time)
	pr.mu.Lock()
	now := time.Now()
	record.LastRefresh = now
	record.NextRefresh = now.Add(record.RefreshInterval)
	record.RefreshCount++
	pr.mu.Unlock()

	if err != nil {
		providerRefreshes.WithLabelValues(cidStr, "failure").Inc()
		pr.logger.Warn("failed to refresh provider record (will retry)",
			zap.String("cid", cidStr),
			zap.Int("refresh_count", record.RefreshCount),
			zap.Error(err),
		)
		return err
	}

	providerRefreshLatency.WithLabelValues(cidStr).Observe(latency)
	providerRefreshes.WithLabelValues(cidStr, "success").Inc()

	pr.logger.Debug("refreshed provider record",
		zap.String("cid", cidStr),
		zap.Int("refresh_count", record.RefreshCount),
		zap.Float64("latency_seconds", latency),
	)

	return nil
}

// ForceRefresh immediately refreshes a specific CID
func (pr *ProviderRefresher) ForceRefresh(ctx context.Context, c cid.Cid) error {
	pr.mu.RLock()
	record, exists := pr.records[c.String()]
	pr.mu.RUnlock()

	if !exists {
		return fmt.Errorf("CID not being provided: %s", c.String())
	}

	return pr.refresh(record)
}

// UpdateInterval changes the refresh interval for a CID
func (pr *ProviderRefresher) UpdateInterval(c cid.Cid, interval time.Duration) error {
	if interval < MinRefreshInterval {
		return fmt.Errorf("interval too small: minimum is %s", MinRefreshInterval)
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	record, exists := pr.records[c.String()]
	if !exists {
		return fmt.Errorf("CID not being provided: %s", c.String())
	}

	record.RefreshInterval = interval
	record.NextRefresh = record.LastRefresh.Add(interval)

	pr.logger.Info("updated refresh interval",
		zap.String("cid", c.String()),
		zap.Duration("interval", interval),
	)

	return nil
}

// Stats returns statistics about provider refreshing
func (pr *ProviderRefresher) Stats() map[string]interface{} {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	totalRefreshes := 0
	for _, record := range pr.records {
		totalRefreshes += record.RefreshCount
	}

	return map[string]interface{}{
		"active_records":  len(pr.records),
		"total_refreshes": totalRefreshes,
		"refresh_interval": pr.interval.String(),
	}
}

// Close stops the provider refresher
func (pr *ProviderRefresher) Close() error {
	pr.cancel()
	pr.ticker.Stop()

	pr.mu.Lock()
	recordCount := len(pr.records)
	pr.records = make(map[string]*ProviderRecord)
	pr.mu.Unlock()

	activeProviderRecords.Set(0)

	pr.logger.Info("provider refresher stopped",
		zap.Int("records_cleared", recordCount),
	)

	return nil
}
