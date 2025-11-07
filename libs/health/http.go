package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Response is the JSON response for health endpoints
type Response struct {
	Status     Status                    `json:"status"`
	Timestamp  time.Time                 `json:"timestamp"`
	Components map[string]CheckResult    `json:"components,omitempty"`
	Metadata   map[string]interface{}    `json:"metadata,omitempty"`
}

// Handler provides HTTP handlers for health checks
type Handler struct {
	health             *Health
	criticalComponents []string
	metadata           map[string]interface{}
}

// NewHandler creates a new health check HTTP handler
func NewHandler(health *Health, opts ...HandlerOption) *Handler {
	h := &Handler{
		health:   health,
		metadata: make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// HandlerOption configures the health handler
type HandlerOption func(*Handler)

// WithCriticalComponents sets the critical components for readiness checks
func WithCriticalComponents(components ...string) HandlerOption {
	return func(h *Handler) {
		h.criticalComponents = components
	}
}

// WithMetadata adds metadata to health responses
func WithMetadata(key string, value interface{}) HandlerOption {
	return func(h *Handler) {
		h.metadata[key] = value
	}
}

// LivenessHandler returns an HTTP handler for /healthz
// Returns 200 if the service is alive (even if degraded)
// Returns 503 if the service is unhealthy
func (h *Handler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		status := h.health.GetStatus(ctx)
		results := h.health.Check(ctx)

		response := Response{
			Status:     status,
			Timestamp:  time.Now(),
			Components: results,
			Metadata:   h.metadata,
		}

		// Liveness: healthy or degraded = 200, unhealthy = 503
		statusCode := http.StatusOK
		if status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler returns an HTTP handler for /readyz
// Returns 200 if the service is ready to accept traffic
// Returns 503 if the service is not ready
func (h *Handler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		ready := h.health.IsReady(ctx, h.criticalComponents)
		results := h.health.Check(ctx)

		status := StatusHealthy
		if !ready {
			status = StatusUnhealthy
		}

		response := Response{
			Status:     status,
			Timestamp:  time.Now(),
			Components: results,
			Metadata:   h.metadata,
		}

		// Readiness: ready = 200, not ready = 503
		statusCode := http.StatusOK
		if !ready {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// DetailedHandler returns a detailed health check (all components)
func (h *Handler) DetailedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		results := h.health.Check(ctx)
		status := h.health.GetStatus(ctx)

		response := Response{
			Status:     status,
			Timestamp:  time.Now(),
			Components: results,
			Metadata:   h.metadata,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// RegisterHandlers registers health check handlers on a mux
func (h *Handler) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.LivenessHandler())
	mux.HandleFunc("/readyz", h.ReadinessHandler())
	mux.HandleFunc("/health", h.DetailedHandler())
}

// QuickHealthz returns a simple 200 OK handler for basic liveness
// This is useful for very lightweight health checks
func QuickHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// QuickReadyz returns a simple readiness check
func QuickReadyz(isReady func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isReady() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
		}
	}
}
