package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	ErrLockAcquisitionFailed = errors.New("failed to acquire lock")
	ErrLockNotHeld           = errors.New("lock not held")
	ErrLockExpired           = errors.New("lock has expired")
	ErrInvalidLockToken      = errors.New("invalid lock token")
	ErrStateUpdateFailed     = errors.New("state update failed")
	ErrStateConflict         = errors.New("state update conflict")
)

// LockType represents the type of distributed lock
type LockType string

const (
	LockTypeExclusive LockType = "exclusive" // Only one holder at a time
	LockTypeShared    LockType = "shared"    // Multiple readers, single writer
)

// Lock represents a distributed lock
type Lock struct {
	ID         string            `json:"id"`
	Resource   string            `json:"resource"` // Resource being locked
	Type       LockType          `json:"type"`
	Holder     string            `json:"holder"` // Agent DID holding the lock
	Token      string            `json:"token"`  // Unique token for this lock acquisition
	AcquiredAt time.Time         `json:"acquired_at"`
	ExpiresAt  time.Time         `json:"expires_at"`
	Renewable  bool              `json:"renewable"`
	Metadata   map[string]string `json:"metadata"`
}

// SharedState represents a piece of shared state in a workflow
type SharedState struct {
	Key       string                 `json:"key"`
	Value     map[string]interface{} `json:"value"`
	Version   int64                  `json:"version"`    // Optimistic locking version
	UpdatedBy string                 `json:"updated_by"` // Agent DID that last updated
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]string      `json:"metadata"`
}

// CoordinationService manages distributed coordination primitives
type CoordinationService struct {
	mu         sync.RWMutex
	messageBus *p2p.MessageBus
	agentID    string
	logger     *zap.Logger

	// Lock management
	locks       map[string]*Lock       // Resource -> Lock
	heldLocks   map[string]*Lock       // Lock tokens held by this service
	lockWaiters map[string][]chan bool // Resource -> waiting channels

	// Shared state management
	sharedState map[string]*SharedState // Key -> State
	stateMu     sync.RWMutex

	// Cleanup
	cleanupTicker *time.Ticker
	stopCh        chan struct{}

	// Metrics
	metricsLocksAcquired  prometheus.Counter
	metricsLocksReleased  prometheus.Counter
	metricsLockConflicts  prometheus.Counter
	metricsLockWaitTime   prometheus.Histogram
	metricsStateUpdates   prometheus.Counter
	metricsStateConflicts prometheus.Counter
}

// NewCoordinationService creates a new coordination service
func NewCoordinationService(
	messageBus *p2p.MessageBus,
	agentID string,
	logger *zap.Logger,
) *CoordinationService {
	cs := &CoordinationService{
		messageBus:  messageBus,
		agentID:     agentID,
		logger:      logger,
		locks:       make(map[string]*Lock),
		heldLocks:   make(map[string]*Lock),
		lockWaiters: make(map[string][]chan bool),
		sharedState: make(map[string]*SharedState),
		stopCh:      make(chan struct{}),

		metricsLocksAcquired: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_coordination_locks_acquired_total",
			Help: "Total number of locks acquired",
		}),
		metricsLocksReleased: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_coordination_locks_released_total",
			Help: "Total number of locks released",
		}),
		metricsLockConflicts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_coordination_lock_conflicts_total",
			Help: "Total number of lock acquisition conflicts",
		}),
		metricsLockWaitTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_coordination_lock_wait_seconds",
			Help:    "Time spent waiting to acquire locks",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12), // 1ms to ~4s
		}),
		metricsStateUpdates: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_coordination_state_updates_total",
			Help: "Total number of shared state updates",
		}),
		metricsStateConflicts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_coordination_state_conflicts_total",
			Help: "Total number of shared state update conflicts",
		}),
	}

	// Start cleanup goroutine
	cs.cleanupTicker = time.NewTicker(5 * time.Second)
	go cs.cleanupExpiredLocks()

	return cs
}

// AcquireLock attempts to acquire a distributed lock
func (cs *CoordinationService) AcquireLock(
	ctx context.Context,
	resource string,
	lockType LockType,
	ttl time.Duration,
) (*Lock, error) {
	startTime := time.Now()
	defer func() {
		cs.metricsLockWaitTime.Observe(time.Since(startTime).Seconds())
	}()

	cs.mu.Lock()

	// Check if lock already exists
	existingLock, exists := cs.locks[resource]
	if exists && time.Now().Before(existingLock.ExpiresAt) {
		// Lock is held
		if existingLock.Type == LockTypeExclusive || lockType == LockTypeExclusive {
			cs.mu.Unlock()
			cs.metricsLockConflicts.Inc()

			// Wait for lock to be released or timeout
			return cs.waitForLock(ctx, resource, lockType, ttl, startTime)
		}

		// Shared lock and existing is also shared - allow
		if existingLock.Type == LockTypeShared && lockType == LockTypeShared {
			cs.mu.Unlock()
			// Create new shared lock entry (simplified - in production would track multiple holders)
			return cs.createLock(resource, lockType, ttl), nil
		}
	}

	cs.mu.Unlock()

	// Acquire lock
	lock := cs.createLock(resource, lockType, ttl)

	cs.mu.Lock()
	cs.locks[resource] = lock
	cs.heldLocks[lock.Token] = lock
	cs.mu.Unlock()

	cs.metricsLocksAcquired.Inc()
	cs.logger.Debug("lock acquired",
		zap.String("resource", resource),
		zap.String("type", string(lockType)),
		zap.String("token", lock.Token),
		zap.Duration("ttl", ttl),
	)

	return lock, nil
}

// createLock creates a new lock instance
func (cs *CoordinationService) createLock(resource string, lockType LockType, ttl time.Duration) *Lock {
	now := time.Now()
	return &Lock{
		ID:         uuid.New().String(),
		Resource:   resource,
		Type:       lockType,
		Holder:     cs.agentID,
		Token:      uuid.New().String(),
		AcquiredAt: now,
		ExpiresAt:  now.Add(ttl),
		Renewable:  true,
		Metadata:   make(map[string]string),
	}
}

// waitForLock waits for a lock to become available
func (cs *CoordinationService) waitForLock(
	ctx context.Context,
	resource string,
	lockType LockType,
	ttl time.Duration,
	startTime time.Time,
) (*Lock, error) {
	// Create waiter channel
	waiterCh := make(chan bool, 1)

	cs.mu.Lock()
	cs.lockWaiters[resource] = append(cs.lockWaiters[resource], waiterCh)
	cs.mu.Unlock()

	defer func() {
		cs.mu.Lock()
		// Remove waiter channel
		waiters := cs.lockWaiters[resource]
		for i, ch := range waiters {
			if ch == waiterCh {
				cs.lockWaiters[resource] = append(waiters[:i], waiters[i+1:]...)
				break
			}
		}
		cs.mu.Unlock()
		close(waiterCh)
	}()

	// Wait for lock or timeout
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrLockAcquisitionFailed, ctx.Err())
	case <-waiterCh:
		// Lock released, try to acquire again
		return cs.AcquireLock(ctx, resource, lockType, ttl)
	case <-time.After(30 * time.Second):
		// Timeout
		return nil, ErrLockAcquisitionFailed
	}
}

// ReleaseLock releases a previously acquired lock
func (cs *CoordinationService) ReleaseLock(token string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	lock, exists := cs.heldLocks[token]
	if !exists {
		return ErrInvalidLockToken
	}

	// Verify lock hasn't expired
	if time.Now().After(lock.ExpiresAt) {
		delete(cs.heldLocks, token)
		delete(cs.locks, lock.Resource)
		return ErrLockExpired
	}

	// Release lock
	delete(cs.heldLocks, token)
	delete(cs.locks, lock.Resource)

	cs.metricsLocksReleased.Inc()
	cs.logger.Debug("lock released",
		zap.String("resource", lock.Resource),
		zap.String("token", token),
	)

	// Notify waiters
	if waiters, ok := cs.lockWaiters[lock.Resource]; ok && len(waiters) > 0 {
		// Notify first waiter
		select {
		case waiters[0] <- true:
		default:
		}
	}

	return nil
}

// RenewLock extends the TTL of an existing lock
func (cs *CoordinationService) RenewLock(token string, ttl time.Duration) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	lock, exists := cs.heldLocks[token]
	if !exists {
		return ErrInvalidLockToken
	}

	if !lock.Renewable {
		return errors.New("lock is not renewable")
	}

	// Verify lock hasn't expired
	if time.Now().After(lock.ExpiresAt) {
		delete(cs.heldLocks, token)
		delete(cs.locks, lock.Resource)
		return ErrLockExpired
	}

	// Renew lock
	lock.ExpiresAt = time.Now().Add(ttl)

	cs.logger.Debug("lock renewed",
		zap.String("resource", lock.Resource),
		zap.String("token", token),
		zap.Duration("new_ttl", ttl),
	)

	return nil
}

// GetState retrieves shared state by key
func (cs *CoordinationService) GetState(key string) (*SharedState, error) {
	cs.stateMu.RLock()
	defer cs.stateMu.RUnlock()

	state, exists := cs.sharedState[key]
	if !exists {
		return nil, fmt.Errorf("state not found: %s", key)
	}

	// Return a copy to prevent external modifications
	stateCopy := *state
	stateCopy.Value = make(map[string]interface{})
	for k, v := range state.Value {
		stateCopy.Value[k] = v
	}

	return &stateCopy, nil
}

// SetState creates or updates shared state (requires optimistic locking)
func (cs *CoordinationService) SetState(
	ctx context.Context,
	key string,
	value map[string]interface{},
	expectedVersion int64,
) (*SharedState, error) {
	cs.stateMu.Lock()
	defer cs.stateMu.Unlock()

	existingState, exists := cs.sharedState[key]

	if exists {
		// Verify version for optimistic locking
		if existingState.Version != expectedVersion {
			cs.metricsStateConflicts.Inc()
			return nil, fmt.Errorf("%w: expected version %d, got %d",
				ErrStateConflict, expectedVersion, existingState.Version)
		}

		// Update existing state
		existingState.Value = value
		existingState.Version++
		existingState.UpdatedBy = cs.agentID
		existingState.UpdatedAt = time.Now()

		cs.metricsStateUpdates.Inc()
		cs.logger.Debug("state updated",
			zap.String("key", key),
			zap.Int64("new_version", existingState.Version),
		)

		return existingState, nil
	}

	// Create new state
	if expectedVersion != 0 {
		return nil, fmt.Errorf("%w: expected version 0 for new state, got %d",
			ErrStateConflict, expectedVersion)
	}

	newState := &SharedState{
		Key:       key,
		Value:     value,
		Version:   1,
		UpdatedBy: cs.agentID,
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}

	cs.sharedState[key] = newState
	cs.metricsStateUpdates.Inc()

	cs.logger.Debug("state created",
		zap.String("key", key),
		zap.Int64("version", newState.Version),
	)

	return newState, nil
}

// UpdateState atomically updates a field in shared state
func (cs *CoordinationService) UpdateState(
	ctx context.Context,
	key string,
	field string,
	value interface{},
) (*SharedState, error) {
	// Retry loop for optimistic locking
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		// Get current state
		currentState, err := cs.GetState(key)
		if err != nil {
			// State doesn't exist, create it
			initialValue := map[string]interface{}{field: value}
			return cs.SetState(ctx, key, initialValue, 0)
		}

		// Update field
		newValue := make(map[string]interface{})
		for k, v := range currentState.Value {
			newValue[k] = v
		}
		newValue[field] = value

		// Try to update with current version
		updatedState, err := cs.SetState(ctx, key, newValue, currentState.Version)
		if err == nil {
			return updatedState, nil
		}

		// Version conflict, retry
		if errors.Is(err, ErrStateConflict) {
			time.Sleep(time.Duration(i+1) * 10 * time.Millisecond) // Exponential backoff
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("%w: max retries exceeded", ErrStateUpdateFailed)
}

// DeleteState removes shared state
func (cs *CoordinationService) DeleteState(key string) error {
	cs.stateMu.Lock()
	defer cs.stateMu.Unlock()

	if _, exists := cs.sharedState[key]; !exists {
		return fmt.Errorf("state not found: %s", key)
	}

	delete(cs.sharedState, key)

	cs.logger.Debug("state deleted", zap.String("key", key))
	return nil
}

// ListState returns all shared state keys
func (cs *CoordinationService) ListState() []string {
	cs.stateMu.RLock()
	defer cs.stateMu.RUnlock()

	keys := make([]string, 0, len(cs.sharedState))
	for key := range cs.sharedState {
		keys = append(keys, key)
	}

	return keys
}

// Barrier implements a distributed barrier for coordination
type Barrier struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	RequiredCount int           `json:"required_count"` // Number of agents required
	Participants  []string      `json:"participants"`   // Agent DIDs
	Timeout       time.Duration `json:"timeout"`
	CreatedAt     time.Time     `json:"created_at"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
}

// WaitAtBarrier waits for all participants to reach the barrier
func (cs *CoordinationService) WaitAtBarrier(
	ctx context.Context,
	barrierName string,
	requiredCount int,
	timeout time.Duration,
) error {
	// This is a simplified barrier implementation using shared state
	barrierKey := fmt.Sprintf("barrier:%s", barrierName)

	// Register at barrier
	for {
		state, err := cs.GetState(barrierKey)
		if err != nil {
			// Barrier doesn't exist, create it
			barrierData := map[string]interface{}{
				"required_count": requiredCount,
				"participants":   []string{cs.agentID},
				"created_at":     time.Now(),
			}
			_, err = cs.SetState(ctx, barrierKey, barrierData, 0)
			if err != nil && !errors.Is(err, ErrStateConflict) {
				return err
			}
			// Conflict means someone else created it, retry
			continue
		}

		// Check if already at barrier
		participants := state.Value["participants"].([]interface{})
		for _, p := range participants {
			if p.(string) == cs.agentID {
				// Already registered, wait for completion
				break
			}
		}

		// Add to participants
		newParticipants := make([]string, len(participants))
		for i, p := range participants {
			newParticipants[i] = p.(string)
		}
		newParticipants = append(newParticipants, cs.agentID)

		barrierData := map[string]interface{}{
			"required_count": requiredCount,
			"participants":   newParticipants,
			"created_at":     state.Value["created_at"],
		}

		_, err = cs.SetState(ctx, barrierKey, barrierData, state.Version)
		if err == nil {
			break // Successfully registered
		}
		if !errors.Is(err, ErrStateConflict) {
			return err
		}
		// Conflict, retry
	}

	// Wait for all participants
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := cs.GetState(barrierKey)
		if err != nil {
			return err
		}

		participants := state.Value["participants"].([]interface{})
		if len(participants) >= requiredCount {
			// Barrier complete
			cs.logger.Info("barrier reached",
				zap.String("barrier", barrierName),
				zap.Int("participants", len(participants)),
			)
			return nil
		}

		// Wait and retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Continue waiting
		}
	}

	return fmt.Errorf("barrier timeout: %s", barrierName)
}

// cleanupExpiredLocks periodically removes expired locks
func (cs *CoordinationService) cleanupExpiredLocks() {
	for {
		select {
		case <-cs.cleanupTicker.C:
			cs.mu.Lock()
			now := time.Now()
			for resource, lock := range cs.locks {
				if now.After(lock.ExpiresAt) {
					delete(cs.locks, resource)
					delete(cs.heldLocks, lock.Token)

					cs.logger.Debug("expired lock removed",
						zap.String("resource", resource),
						zap.String("token", lock.Token),
					)

					// Notify waiters
					if waiters, ok := cs.lockWaiters[resource]; ok && len(waiters) > 0 {
						select {
						case waiters[0] <- true:
						default:
						}
					}
				}
			}
			cs.mu.Unlock()

		case <-cs.stopCh:
			cs.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the coordination service
func (cs *CoordinationService) Stop() {
	close(cs.stopCh)

	// Release all held locks
	cs.mu.Lock()
	for token := range cs.heldLocks {
		cs.ReleaseLock(token)
	}
	cs.mu.Unlock()

	cs.logger.Info("coordination service stopped")
}

// BroadcastCoordinationMessage broadcasts a coordination message to all agents
func (cs *CoordinationService) BroadcastCoordinationMessage(
	ctx context.Context,
	messageType string,
	payload map[string]interface{},
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal coordination message: %w", err)
	}

	return cs.messageBus.Broadcast(ctx, payloadJSON, messageType)
}
