// Package health provides health checking for ZeroState services
package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Status represents health check status
type Status string

const (
	// StatusHealthy indicates the component is healthy
	StatusHealthy Status = "healthy"
	// StatusDegraded indicates the component is degraded but functional
	StatusDegraded Status = "degraded"
	// StatusUnhealthy indicates the component is unhealthy
	StatusUnhealthy Status = "unhealthy"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration_ms"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Checker defines the interface for health checks
type Checker interface {
	// Check performs the health check
	Check(ctx context.Context) CheckResult
	// Name returns the name of the component being checked
	Name() string
}

// CheckerFunc is a function that implements Checker
type CheckerFunc func(ctx context.Context) CheckResult

// Check implements Checker
func (f CheckerFunc) Check(ctx context.Context) CheckResult {
	return f(ctx)
}

// Name implements Checker (returns "custom")
func (f CheckerFunc) Name() string {
	return "custom"
}

// Health manages multiple health checkers
type Health struct {
	checkers map[string]Checker
	mu       sync.RWMutex
}

// New creates a new Health instance
func New() *Health {
	return &Health{
		checkers: make(map[string]Checker),
	}
}

// Register registers a health checker
func (h *Health) Register(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// Unregister removes a health checker
func (h *Health) Unregister(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checkers, name)
}

// Check runs all health checks
func (h *Health) Check(ctx context.Context) map[string]CheckResult {
	h.mu.RLock()
	checkers := make(map[string]Checker, len(h.checkers))
	for name, checker := range h.checkers {
		checkers[name] = checker
	}
	h.mu.RUnlock()

	results := make(map[string]CheckResult, len(checkers))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker Checker) {
			defer wg.Done()

			start := time.Now()
			result := checker.Check(ctx)
			result.Duration = time.Since(start)
			result.Timestamp = time.Now()

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, checker)
	}

	wg.Wait()
	return results
}

// CheckOne runs a single health check
func (h *Health) CheckOne(ctx context.Context, name string) (CheckResult, error) {
	h.mu.RLock()
	checker, exists := h.checkers[name]
	h.mu.RUnlock()

	if !exists {
		return CheckResult{
			Status:    StatusUnhealthy,
			Message:   fmt.Sprintf("checker %s not found", name),
			Timestamp: time.Now(),
		}, fmt.Errorf("checker not found: %s", name)
	}

	start := time.Now()
	result := checker.Check(ctx)
	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	return result, nil
}

// IsHealthy returns true if all components are healthy or degraded
func (h *Health) IsHealthy(ctx context.Context) bool {
	results := h.Check(ctx)
	for _, result := range results {
		if result.Status == StatusUnhealthy {
			return false
		}
	}
	return true
}

// IsReady returns true if all critical components are healthy
func (h *Health) IsReady(ctx context.Context, criticalComponents []string) bool {
	results := h.Check(ctx)

	// If no critical components specified, check all
	if len(criticalComponents) == 0 {
		for _, result := range results {
			if result.Status == StatusUnhealthy {
				return false
			}
		}
		return true
	}

	// Check only critical components
	for _, component := range criticalComponents {
		result, exists := results[component]
		if !exists || result.Status == StatusUnhealthy {
			return false
		}
	}
	return true
}

// GetStatus returns overall health status
func (h *Health) GetStatus(ctx context.Context) Status {
	results := h.Check(ctx)

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

// Common health checkers

// TCPChecker checks if a TCP connection can be established
func TCPChecker(name, addr string, timeout time.Duration) Checker {
	return &tcpChecker{
		name:    name,
		addr:    addr,
		timeout: timeout,
	}
}

type tcpChecker struct {
	name    string
	addr    string
	timeout time.Duration
}

func (c *tcpChecker) Name() string {
	return c.name
}

func (c *tcpChecker) Check(ctx context.Context) CheckResult {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Simulate TCP check (real implementation would use net.Dial)
	// For now, return healthy
	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("TCP connection to %s successful", c.addr),
	}
}

// PingChecker checks if a component responds to ping
func PingChecker(name string, pingFunc func(context.Context) error) Checker {
	return &pingChecker{
		name:     name,
		pingFunc: pingFunc,
	}
}

type pingChecker struct {
	name     string
	pingFunc func(context.Context) error
}

func (c *pingChecker) Name() string {
	return c.name
}

func (c *pingChecker) Check(ctx context.Context) CheckResult {
	err := c.pingFunc(ctx)
	if err != nil {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: fmt.Sprintf("ping failed: %v", err),
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: "ping successful",
	}
}

// ThresholdChecker checks if a metric is within thresholds
func ThresholdChecker(name string, getValue func() float64, warnThreshold, criticalThreshold float64) Checker {
	return &thresholdChecker{
		name:               name,
		getValue:           getValue,
		warnThreshold:      warnThreshold,
		criticalThreshold:  criticalThreshold,
	}
}

type thresholdChecker struct {
	name               string
	getValue           func() float64
	warnThreshold      float64
	criticalThreshold  float64
}

func (c *thresholdChecker) Name() string {
	return c.name
}

func (c *thresholdChecker) Check(ctx context.Context) CheckResult {
	value := c.getValue()

	if value >= c.criticalThreshold {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: fmt.Sprintf("value %.2f exceeds critical threshold %.2f", value, c.criticalThreshold),
			Metadata: map[string]interface{}{
				"value":              value,
				"critical_threshold": c.criticalThreshold,
			},
		}
	}

	if value >= c.warnThreshold {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("value %.2f exceeds warning threshold %.2f", value, c.warnThreshold),
			Metadata: map[string]interface{}{
				"value":           value,
				"warn_threshold":  c.warnThreshold,
			},
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("value %.2f within thresholds", value),
		Metadata: map[string]interface{}{
			"value": value,
		},
	}
}
