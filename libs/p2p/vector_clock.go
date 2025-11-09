package p2p

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/aidenlippert/zerostate/libs/identity"
)

var (
	vectorClockConflicts = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "zerostate_vector_clock_conflicts_total",
			Help: "Total number of vector clock conflicts detected",
		},
	)

	vectorClockOrderingViolations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "zerostate_vector_clock_ordering_violations_total",
			Help: "Total number of causal ordering violations detected",
		},
	)

	vectorClockMerges = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "zerostate_vector_clock_merges_total",
			Help: "Total number of vector clock merges performed",
		},
	)
)

// VectorClock implements a vector clock for causal ordering
type VectorClock struct {
	mu      sync.RWMutex
	Clocks  map[string]uint64 `json:"clocks"`  // peer.ID -> logical time
	Version uint64            `json:"version"` // monotonic version for comparison
}

// NewVectorClock creates a new vector clock
func NewVectorClock() *VectorClock {
	return &VectorClock{
		Clocks:  make(map[string]uint64),
		Version: 0,
	}
}

// Increment increments the clock for a given peer
func (vc *VectorClock) Increment(peerID peer.ID) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	pid := peerID.String()
	vc.Clocks[pid]++
	vc.Version++
}

// Update updates the clock based on a received clock
func (vc *VectorClock) Update(peerID peer.ID, received *VectorClock) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	pid := peerID.String()

	// Merge: take max of each clock value
	for p, t := range received.Clocks {
		if current, exists := vc.Clocks[p]; !exists || t > current {
			vc.Clocks[p] = t
		}
	}

	// Increment own clock
	vc.Clocks[pid]++
	vc.Version++

	vectorClockMerges.Inc()
}

// HappensBefore checks if this clock happens before another
func (vc *VectorClock) HappensBefore(other *VectorClock) bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if other == nil {
		return false
	}

	hasSmaller := false
	for p, t := range vc.Clocks {
		otherT, exists := other.Clocks[p]
		if !exists || t > otherT {
			return false // Not happened before if any clock is greater
		}
		if t < otherT {
			hasSmaller = true
		}
	}

	return hasSmaller
}

// ConcurrentWith checks if two clocks are concurrent (conflicting)
func (vc *VectorClock) ConcurrentWith(other *VectorClock) bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if other == nil {
		return false
	}

	return !vc.happensBefore(other) && !other.happensBefore(vc)
}

// happensBefore is internal version without locking
func (vc *VectorClock) happensBefore(other *VectorClock) bool {
	if other == nil {
		return false
	}

	hasSmaller := false
	for p, t := range vc.Clocks {
		otherT, exists := other.Clocks[p]
		if !exists || t > otherT {
			return false
		}
		if t < otherT {
			hasSmaller = true
		}
	}

	return hasSmaller
}

// Copy creates a deep copy of the vector clock
func (vc *VectorClock) Copy() *VectorClock {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	clocks := make(map[string]uint64, len(vc.Clocks))
	for p, t := range vc.Clocks {
		clocks[p] = t
	}

	return &VectorClock{
		Clocks:  clocks,
		Version: vc.Version,
	}
}

// String returns a string representation
func (vc *VectorClock) String() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	b, _ := json.Marshal(vc.Clocks)
	return fmt.Sprintf("VC{v=%d, %s}", vc.Version, string(b))
}

// CardUpdate represents a versioned agent card update
type CardUpdate struct {
	Card       *identity.AgentCard `json:"card"`        // The agent card data
	Clock      *VectorClock        `json:"clock"`       // Vector clock for causal ordering
	PrevHash   string              `json:"prev_hash"`   // Hash of previous version (chain)
	Signature  []byte              `json:"signature"`   // Ed25519 signature
	UpdaterID  string              `json:"updater_id"`  // Peer who created this update
	Timestamp  int64               `json:"timestamp"`   // Unix timestamp
}

// UpdateHistory tracks the history of card updates with causal ordering
type UpdateHistory struct {
	mu       sync.RWMutex
	updates  map[string]*CardUpdate // hash -> update
	latest   *CardUpdate            // current latest version
	conflicts []ConflictPair        // detected conflicts
}

// ConflictPair represents two concurrent updates
type ConflictPair struct {
	Update1 *CardUpdate
	Update2 *CardUpdate
}

// NewUpdateHistory creates a new update history tracker
func NewUpdateHistory() *UpdateHistory {
	return &UpdateHistory{
		updates:   make(map[string]*CardUpdate),
		conflicts: make([]ConflictPair, 0),
	}
}

// AddUpdate adds a new update to the history
func (uh *UpdateHistory) AddUpdate(update *CardUpdate) error {
	uh.mu.Lock()
	defer uh.mu.Unlock()

	// Check causal ordering
	if uh.latest != nil {
		if update.Clock.HappensBefore(uh.latest.Clock) {
			vectorClockOrderingViolations.Inc()
			return fmt.Errorf("ordering violation: update is causally before current latest")
		}

		// Detect conflict (concurrent updates)
		if update.Clock.ConcurrentWith(uh.latest.Clock) {
			vectorClockConflicts.Inc()
			uh.conflicts = append(uh.conflicts, ConflictPair{
				Update1: uh.latest,
				Update2: update,
			})
			
			// Last-Write-Wins resolution: use timestamp
			if update.Timestamp > uh.latest.Timestamp {
				uh.latest = update
			}
			return nil
		}
	}

	// Store update
	hash := computeUpdateHash(update)
	uh.updates[hash] = update

	// Update latest if causally after
	if uh.latest == nil || uh.latest.Clock.HappensBefore(update.Clock) {
		uh.latest = update
	}

	return nil
}

// GetLatest returns the current latest update
func (uh *UpdateHistory) GetLatest() *CardUpdate {
	uh.mu.RLock()
	defer uh.mu.RUnlock()
	return uh.latest
}

// GetConflicts returns all detected conflicts
func (uh *UpdateHistory) GetConflicts() []ConflictPair {
	uh.mu.RLock()
	defer uh.mu.RUnlock()
	
	conflicts := make([]ConflictPair, len(uh.conflicts))
	copy(conflicts, uh.conflicts)
	return conflicts
}

// HasConflicts checks if there are any unresolved conflicts
func (uh *UpdateHistory) HasConflicts() bool {
	uh.mu.RLock()
	defer uh.mu.RUnlock()
	return len(uh.conflicts) > 0
}

// computeUpdateHash computes a hash of the update for deduplication
func computeUpdateHash(update *CardUpdate) string {
	// Simple hash based on DID + timestamp + clock version
	return fmt.Sprintf("%s-%d-%d", 
		update.Card.DID, 
		update.Timestamp, 
		update.Clock.Version,
	)
}

// ValidateOrdering validates that an update respects causal ordering
func ValidateOrdering(current, incoming *CardUpdate) error {
	if current == nil {
		return nil // First update is always valid
	}

	// Check clock relationship
	if incoming.Clock.HappensBefore(current.Clock) {
		vectorClockOrderingViolations.Inc()
		return fmt.Errorf("causal ordering violation: incoming update is before current")
	}

	// Check chain continuity
	if incoming.PrevHash != "" && incoming.PrevHash != computeUpdateHash(current) {
		return fmt.Errorf("chain break: prev_hash does not match current update")
	}

	return nil
}

// MergeClocks merges two vector clocks (for gossip protocols)
func MergeClocks(vc1, vc2 *VectorClock) *VectorClock {
	if vc1 == nil {
		return vc2.Copy()
	}
	if vc2 == nil {
		return vc1.Copy()
	}

	merged := NewVectorClock()
	
	// Take max of all clocks
	allPeers := make(map[string]bool)
	for p := range vc1.Clocks {
		allPeers[p] = true
	}
	for p := range vc2.Clocks {
		allPeers[p] = true
	}

	for p := range allPeers {
		t1 := vc1.Clocks[p]
		t2 := vc2.Clocks[p]
		if t1 > t2 {
			merged.Clocks[p] = t1
		} else {
			merged.Clocks[p] = t2
		}
	}

	// Version is max of both
	if vc1.Version > vc2.Version {
		merged.Version = vc1.Version
	} else {
		merged.Version = vc2.Version
	}

	vectorClockMerges.Inc()
	return merged
}
