package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Namespace for all ZeroState metrics
	Namespace = "zerostate"
)

// Registry wraps prometheus.Registry with ZeroState-specific helpers
type Registry struct {
	reg *prometheus.Registry
	mu  sync.RWMutex

	// Standard collectors
	counters   map[string]*prometheus.CounterVec
	gauges     map[string]*prometheus.GaugeVec
	histograms map[string]*prometheus.HistogramVec
	summaries  map[string]*prometheus.SummaryVec
}

// NewRegistry creates a new metrics registry
func NewRegistry() *Registry {
	return &Registry{
		reg:        prometheus.NewRegistry(),
		counters:   make(map[string]*prometheus.CounterVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		summaries:  make(map[string]*prometheus.SummaryVec),
	}
}

// DefaultRegistry is the global registry used by default
var (
	defaultRegistry     *Registry
	defaultRegistryOnce sync.Once
)

// Default returns the default global registry
func Default() *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRegistry()
	})
	return defaultRegistry
}

// Counter creates or retrieves a counter metric
func (r *Registry) Counter(name, help string, labels ...string) *prometheus.CounterVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	if counter, exists := r.counters[name]; exists {
		return counter
	}

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      name,
			Help:      help,
		},
		labels,
	)

	r.reg.MustRegister(counter)
	r.counters[name] = counter
	return counter
}

// Gauge creates or retrieves a gauge metric
func (r *Registry) Gauge(name, help string, labels ...string) *prometheus.GaugeVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	if gauge, exists := r.gauges[name]; exists {
		return gauge
	}

	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      name,
			Help:      help,
		},
		labels,
	)

	r.reg.MustRegister(gauge)
	r.gauges[name] = gauge
	return gauge
}

// Histogram creates or retrieves a histogram metric
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	if histogram, exists := r.histograms[name]; exists {
		return histogram
	}

	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      name,
			Help:      help,
			Buckets:   buckets,
		},
		labels,
	)

	r.reg.MustRegister(histogram)
	r.histograms[name] = histogram
	return histogram
}

// Summary creates or retrieves a summary metric
func (r *Registry) Summary(name, help string, objectives map[float64]float64, labels ...string) *prometheus.SummaryVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	if summary, exists := r.summaries[name]; exists {
		return summary
	}

	summary := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  Namespace,
			Name:       name,
			Help:       help,
			Objectives: objectives,
		},
		labels,
	)

	r.reg.MustRegister(summary)
	r.summaries[name] = summary
	return summary
}

// Handler returns an HTTP handler for metrics exposition
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// MustRegister registers a collector and panics if there's an error
func (r *Registry) MustRegister(collectors ...prometheus.Collector) {
	r.reg.MustRegister(collectors...)
}

// Unregister removes a collector from the registry
func (r *Registry) Unregister(collector prometheus.Collector) bool {
	return r.reg.Unregister(collector)
}

// Standard bucket definitions for common use cases
var (
	// DurationBuckets for measuring operation durations (microseconds to seconds)
	DurationBuckets = []float64{
		0.0001, // 100µs
		0.0005, // 500µs
		0.001,  // 1ms
		0.005,  // 5ms
		0.01,   // 10ms
		0.05,   // 50ms
		0.1,    // 100ms
		0.5,    // 500ms
		1.0,    // 1s
		5.0,    // 5s
		10.0,   // 10s
	}

	// BytesBuckets for measuring data sizes (bytes to gigabytes)
	BytesBuckets = []float64{
		1024,             // 1KB
		10 * 1024,        // 10KB
		100 * 1024,       // 100KB
		1024 * 1024,      // 1MB
		10 * 1024 * 1024, // 10MB
		100 * 1024 * 1024,                  // 100MB
		1024 * 1024 * 1024,                 // 1GB
	}

	// CostBuckets for measuring task costs (units)
	CostBuckets = []float64{
		0.001, 0.01, 0.1, 1.0, 10.0, 100.0, 1000.0,
	}

	// CountBuckets for measuring counts (peers, tasks, etc.)
	CountBuckets = []float64{
		1, 5, 10, 25, 50, 100, 250, 500, 1000,
	}

	// DefaultQuantiles for summary metrics
	DefaultQuantiles = map[float64]float64{
		0.5:  0.05,  // median with 5% error
		0.9:  0.01,  // 90th percentile with 1% error
		0.99: 0.001, // 99th percentile with 0.1% error
	}
)
