package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/aidenlippert/zerostate/libs/identity"
)

func TestNewVectorClock(t *testing.T) {
	vc := NewVectorClock()
	assert.NotNil(t, vc)
	assert.Equal(t, uint64(0), vc.Version)
	assert.Empty(t, vc.Clocks)
}

func TestVectorClockIncrement(t *testing.T) {
	vc := NewVectorClock()
	peerID := peer.ID("peer1")

	vc.Increment(peerID)
	assert.Equal(t, uint64(1), vc.Clocks[peerID.String()])
	assert.Equal(t, uint64(1), vc.Version)

	vc.Increment(peerID)
	assert.Equal(t, uint64(2), vc.Clocks[peerID.String()])
	assert.Equal(t, uint64(2), vc.Version)
}

func TestVectorClockUpdate(t *testing.T) {
	vc1 := NewVectorClock()
	vc2 := NewVectorClock()

	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")

	// peer1 increments twice
	vc1.Increment(peer1)
	vc1.Increment(peer1)

	// peer2 increments once
	vc2.Increment(peer2)

	// peer2 receives update from peer1
	vc2.Update(peer2, vc1)

	// peer2 should have max of both clocks plus own increment
	assert.Equal(t, uint64(2), vc2.Clocks[peer1.String()]) // From vc1
	assert.Equal(t, uint64(2), vc2.Clocks[peer2.String()]) // 1 (original) + 1 (update increment)
}

func TestHappensBefore(t *testing.T) {
	vc1 := NewVectorClock()
	vc2 := NewVectorClock()

	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")

	vc1.Increment(peer1)
	vc1.Clocks[peer1.String()] = 1

	vc2.Increment(peer2)
	vc2.Clocks[peer1.String()] = 2
	vc2.Clocks[peer2.String()] = 1

	// vc1 happens before vc2
	assert.True(t, vc1.HappensBefore(vc2))
	assert.False(t, vc2.HappensBefore(vc1))
}

func TestConcurrentClocks(t *testing.T) {
	vc1 := NewVectorClock()
	vc2 := NewVectorClock()

	// Create concurrent updates
	vc1.Clocks["peer1"] = 2
	vc1.Clocks["peer2"] = 0

	vc2.Clocks["peer1"] = 0
	vc2.Clocks["peer2"] = 2

	// Both are concurrent
	assert.True(t, vc1.ConcurrentWith(vc2))
	assert.True(t, vc2.ConcurrentWith(vc1))
	assert.False(t, vc1.HappensBefore(vc2))
	assert.False(t, vc2.HappensBefore(vc1))
}

func TestVectorClockCopy(t *testing.T) {
	vc1 := NewVectorClock()
	peer1 := peer.ID("peer1")

	vc1.Increment(peer1)
	vc1.Increment(peer1)

	vc2 := vc1.Copy()

	// Should be equal but independent
	assert.Equal(t, vc1.Clocks[peer1.String()], vc2.Clocks[peer1.String()])
	assert.Equal(t, vc1.Version, vc2.Version)

	// Modifying copy should not affect original
	vc2.Increment(peer1)
	assert.NotEqual(t, vc1.Clocks[peer1.String()], vc2.Clocks[peer1.String()])
}

func TestMergeClocks(t *testing.T) {
	vc1 := NewVectorClock()
	vc2 := NewVectorClock()

	vc1.Clocks["peer1"] = 5
	vc1.Clocks["peer2"] = 2
	vc1.Version = 7

	vc2.Clocks["peer1"] = 3
	vc2.Clocks["peer2"] = 4
	vc2.Clocks["peer3"] = 1
	vc2.Version = 8

	merged := MergeClocks(vc1, vc2)

	// Should take max of each clock
	assert.Equal(t, uint64(5), merged.Clocks["peer1"])
	assert.Equal(t, uint64(4), merged.Clocks["peer2"])
	assert.Equal(t, uint64(1), merged.Clocks["peer3"])
	assert.Equal(t, uint64(8), merged.Version)
}

func TestNewUpdateHistory(t *testing.T) {
	history := NewUpdateHistory()
	assert.NotNil(t, history)
	assert.Nil(t, history.GetLatest())
	assert.False(t, history.HasConflicts())
}

func TestAddUpdateFirstUpdate(t *testing.T) {
	history := NewUpdateHistory()
	
	card := &identity.AgentCard{
		DID: "did:key:z6MkTest1",
		Endpoints: &identity.Endpoints{
			Libp2p: []string{"/ip4/127.0.0.1/tcp/4001"},
		},
	}

	vc := NewVectorClock()
	vc.Increment(peer.ID("peer1"))

	update := &CardUpdate{
		Card:      card,
		Clock:     vc,
		PrevHash:  "",
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix(),
	}

	err := history.AddUpdate(update)
	require.NoError(t, err)
	assert.Equal(t, update, history.GetLatest())
}

func TestAddUpdateCausalOrder(t *testing.T) {
	history := NewUpdateHistory()
	peer1 := peer.ID("peer1")

	// First update
	vc1 := NewVectorClock()
	vc1.Increment(peer1)
	update1 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4001"},
			},
		},
		Clock:     vc1,
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix(),
	}
	history.AddUpdate(update1)

	// Second update (causally after)
	vc2 := vc1.Copy()
	vc2.Increment(peer1)
	update2 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4002"},
			},
		},
		Clock:     vc2,
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix() + 1,
	}

	err := history.AddUpdate(update2)
	require.NoError(t, err)
	assert.Equal(t, update2, history.GetLatest())
}

func TestAddUpdateOrderingViolation(t *testing.T) {
	history := NewUpdateHistory()
	peer1 := peer.ID("peer1")

	// First update
	vc1 := NewVectorClock()
	vc1.Increment(peer1)
	vc1.Increment(peer1)
	update1 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4001"},
			},
		},
		Clock:     vc1,
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix(),
	}
	history.AddUpdate(update1)

	// Second update (causally before - violation)
	vc2 := NewVectorClock()
	vc2.Increment(peer1)
	update2 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4002"},
			},
		},
		Clock:     vc2,
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix() + 1,
	}

	err := history.AddUpdate(update2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ordering violation")
	assert.Equal(t, update1, history.GetLatest()) // Should keep original
}

func TestAddUpdateConflict(t *testing.T) {
	history := NewUpdateHistory()
	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")

	// First update from peer1
	vc1 := NewVectorClock()
	vc1.Increment(peer1)
	update1 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4001"},
			},
		},
		Clock:     vc1,
		UpdaterID: "peer1",
		Timestamp: 1000,
	}
	history.AddUpdate(update1)

	// Concurrent update from peer2
	vc2 := NewVectorClock()
	vc2.Increment(peer2)
	update2 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4002"},
			},
		},
		Clock:     vc2,
		UpdaterID: "peer2",
		Timestamp: 2000, // Later timestamp
	}

	err := history.AddUpdate(update2)
	require.NoError(t, err) // No error, but conflict detected

	// Should have conflict recorded
	assert.True(t, history.HasConflicts())
	conflicts := history.GetConflicts()
	assert.Len(t, conflicts, 1)

	// LWW resolution: update2 wins (later timestamp)
	assert.Equal(t, update2, history.GetLatest())
}

func TestValidateOrdering(t *testing.T) {
	peer1 := peer.ID("peer1")

	// First update
	vc1 := NewVectorClock()
	vc1.Increment(peer1)
	update1 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4001"},
			},
		},
		Clock:     vc1,
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix(),
	}

	// Valid next update
	vc2 := vc1.Copy()
	vc2.Increment(peer1)
	update2 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4002"},
			},
		},
		Clock:     vc2,
		PrevHash:  computeUpdateHash(update1),
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix() + 1,
	}

	err := ValidateOrdering(update1, update2)
	assert.NoError(t, err)
}

func TestValidateOrderingViolation(t *testing.T) {
	peer1 := peer.ID("peer1")

	// Later update
	vc1 := NewVectorClock()
	vc1.Increment(peer1)
	vc1.Increment(peer1)
	update1 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4001"},
			},
		},
		Clock:     vc1,
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix(),
	}

	// Earlier update (violation)
	vc2 := NewVectorClock()
	vc2.Increment(peer1)
	update2 := &CardUpdate{
		Card: &identity.AgentCard{
			DID: "did:key:z6MkTest1",
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/4002"},
			},
		},
		Clock:     vc2,
		UpdaterID: "peer1",
		Timestamp: time.Now().Unix() + 1,
	}

	err := ValidateOrdering(update1, update2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "causal ordering violation")
}
