package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HTTPMetrics tracks HTTP server metrics
type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
	activeRequests  *prometheus.GaugeVec
}

// NewHTTPMetrics creates HTTP metrics collectors
func NewHTTPMetrics(registry *Registry) *HTTPMetrics {
	return &HTTPMetrics{
		requestsTotal: registry.Counter(
			"http_requests_total",
			"Total HTTP requests processed",
			"method", "path", "status",
		),
		requestDuration: registry.Histogram(
			"http_request_duration_seconds",
			"HTTP request duration in seconds",
			DurationBuckets,
			"method", "path",
		),
		requestSize: registry.Histogram(
			"http_request_size_bytes",
			"HTTP request size in bytes",
			BytesBuckets,
			"method", "path",
		),
		responseSize: registry.Histogram(
			"http_response_size_bytes",
			"HTTP response size in bytes",
			BytesBuckets,
			"method", "path",
		),
		activeRequests: registry.Gauge(
			"http_requests_active",
			"Number of active HTTP requests",
			"method", "path",
		),
	}
}

// Middleware returns an HTTP middleware that tracks metrics
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := r.URL.Path
		method := r.Method

		// Track active requests
		m.activeRequests.WithLabelValues(method, path).Inc()
		defer m.activeRequests.WithLabelValues(method, path).Dec()

		// Track request size
		if r.ContentLength > 0 {
			m.requestSize.WithLabelValues(method, path).Observe(float64(r.ContentLength))
		}

		// Wrap response writer to capture status and size
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		m.requestDuration.WithLabelValues(method, path).Observe(duration)
		m.requestsTotal.WithLabelValues(method, path, http.StatusText(wrapped.statusCode)).Inc()
		m.responseSize.WithLabelValues(method, path).Observe(float64(wrapped.size))
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// MetricsServer creates an HTTP server for metrics exposition
type MetricsServer struct {
	registry *Registry
	server   *http.Server
	metrics  *HTTPMetrics
}

// NewMetricsServer creates a new metrics HTTP server
func NewMetricsServer(addr string, registry *Registry) *MetricsServer {
	if registry == nil {
		registry = Default()
	}

	metrics := NewHTTPMetrics(registry)
	mux := http.NewServeMux()
	
	// Metrics endpoint
	mux.Handle("/metrics", registry.Handler())
	
	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return &MetricsServer{
		registry: registry,
		server: &http.Server{
			Addr:         addr,
			Handler:      metrics.Middleware(mux),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		metrics: metrics,
	}
}

// Start starts the metrics server
func (s *MetricsServer) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully stops the metrics server
func (s *MetricsServer) Stop() error {
	return s.server.Close()
}
