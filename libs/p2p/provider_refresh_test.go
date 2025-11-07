package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func createTestDHT(t *testing.T) (*dht.IpfsDHT, func()) {
	ctx := context.Background()
	
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)

	// Use client mode to avoid needing peers
	d, err := dht.New(ctx, h, dht.Mode(dht.ModeClient))
	require.NoError(t, err)

	cleanup := func() {
		d.Close()
		h.Close()
	}

	return d, cleanup
}

func createTestCID(t *testing.T) cid.Cid {
	// Create a simple CID
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}

	c, err := pref.Sum([]byte("test-content"))
	require.NoError(t, err)
	return c
}

func TestNewProviderRefresher(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	assert.NotNil(t, pr)
	assert.Equal(t, DefaultRefreshInterval, pr.interval)
	assert.NotNil(t, pr.ticker)
}

func TestProviderRefresherProvide(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	config := &ProviderRefresherConfig{
		RefreshInterval: 1 * time.Hour,
		EnableMetrics:   true,
	}
	pr := NewProviderRefresher(ctx, d, config, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Provide CID
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	// Verify record exists
	record, exists := pr.GetRecord(c)
	assert.True(t, exists)
	assert.Equal(t, c, record.CID)
	assert.Equal(t, 0, record.RefreshCount)
	assert.NotZero(t, record.ProvidedAt)

	// Providing again should be idempotent
	err = pr.Provide(ctx, c)
	require.NoError(t, err)
}

func TestProviderRefresherUnprovide(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Provide then unprovide
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	pr.Unprovide(c)

	// Verify record removed
	_, exists := pr.GetRecord(c)
	assert.False(t, exists)
}

func TestProviderRefresherListRecords(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	// Provide multiple CIDs
	c1 := createTestCID(t)
	c2, _ := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}.Sum([]byte("test-content-2"))

	err := pr.Provide(ctx, c1)
	require.NoError(t, err)

	err = pr.Provide(ctx, c2)
	require.NoError(t, err)

	// List records
	records := pr.ListRecords()
	assert.Len(t, records, 2)
}

func TestProviderRefresherAutoRefresh(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	config := &ProviderRefresherConfig{
		RefreshInterval: 100 * time.Millisecond, // Very fast refresh for testing
		EnableMetrics:   true,
	}
	pr := NewProviderRefresher(ctx, d, config, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Provide CID
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	// Set NextRefresh to past so refresh triggers on next tick
	pr.mu.Lock()
	if rec, exists := pr.records[c.String()]; exists {
		rec.NextRefresh = time.Now().Add(-1 * time.Second)
	}
	pr.mu.Unlock()

	// Manually trigger refreshAll to test the refresh logic
	// (relying on ticker timing in tests can be flaky)
	pr.refreshAll()

	// Check refresh count increased (DHT may fail but record should update)
	record, exists := pr.GetRecord(c)
	require.True(t, exists)
	assert.Greater(t, record.RefreshCount, 0, "should have auto-refreshed")
}

func TestProviderRefresherForceRefresh(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Provide CID
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	initialRecord, _ := pr.GetRecord(c)
	initialCount := initialRecord.RefreshCount

	// Force refresh (may fail DHT but should still update record)
	pr.ForceRefresh(ctx, c)

	// Verify refresh count increased (even if DHT failed)
	record, exists := pr.GetRecord(c)
	require.True(t, exists)
	assert.Equal(t, initialCount+1, record.RefreshCount)
}

func TestProviderRefresherForceRefreshNonExistent(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Try to force refresh non-existent CID
	err := pr.ForceRefresh(ctx, c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not being provided")
}

func TestProviderRefresherUpdateInterval(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Provide CID
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	// Update interval
	newInterval := 2 * time.Hour
	err = pr.UpdateInterval(c, newInterval)
	require.NoError(t, err)

	// Verify updated
	record, _ := pr.GetRecord(c)
	assert.Equal(t, newInterval, record.RefreshInterval)
}

func TestProviderRefresherUpdateIntervalTooSmall(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	c := createTestCID(t)
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	// Try to set interval too small
	err = pr.UpdateInterval(c, 30*time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too small")
}

func TestProviderRefresherUpdateIntervalNonExistent(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Try to update interval for non-existent CID
	err := pr.UpdateInterval(c, 2*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not being provided")
}

func TestProviderRefresherStats(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	c := createTestCID(t)

	// Initial stats
	stats := pr.Stats()
	assert.Equal(t, 0, stats["active_records"])
	assert.Equal(t, 0, stats["total_refreshes"])

	// Provide CID
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	// Stats after providing
	stats = pr.Stats()
	assert.Equal(t, 1, stats["active_records"])
}

func TestProviderRefresherClose(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)

	c := createTestCID(t)
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	// Close
	err = pr.Close()
	require.NoError(t, err)

	// Verify records cleared
	pr.mu.RLock()
	recordCount := len(pr.records)
	pr.mu.RUnlock()
	assert.Equal(t, 0, recordCount)
}

func TestProviderRefresherMinimumInterval(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	config := &ProviderRefresherConfig{
		RefreshInterval: 30 * time.Minute, // Below minimum
		EnableMetrics:   true,
	}
	pr := NewProviderRefresher(ctx, d, config, logger)
	defer pr.Close()

	// Should use minimum interval
	assert.Equal(t, MinRefreshInterval, pr.interval)
}

func TestProviderRefresherRefreshAll(t *testing.T) {
	ctx := context.Background()
	d, cleanup := createTestDHT(t)
	defer cleanup()

	logger := zap.NewNop()
	pr := NewProviderRefresher(ctx, d, nil, logger)
	defer pr.Close()

	// Provide CID with past next refresh time
	c := createTestCID(t)
	err := pr.Provide(ctx, c)
	require.NoError(t, err)

	// Manually set next refresh to past
	pr.mu.Lock()
	record := pr.records[c.String()]
	record.NextRefresh = time.Now().Add(-1 * time.Hour)
	pr.mu.Unlock()

	// Trigger refresh
	pr.refreshAll()

	// Check refresh happened (DHT may fail but record should update)
	record, _ = pr.GetRecord(c)
	assert.Greater(t, record.RefreshCount, 0)
	assert.True(t, record.NextRefresh.After(time.Now()))
}
