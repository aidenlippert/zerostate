package economic

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/aidenlippert/zerostate/libs/metrics"
)

func TestNewEconomicMetrics(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	require.NotNil(t, m)
	assert.NotNil(t, m.ChannelsTotal)
	assert.NotNil(t, m.PaymentsTotal)
	assert.NotNil(t, m.ReputationScores)
	assert.NotNil(t, m.SettlementsTotal)
	assert.NotNil(t, m.TotalValueLocked)
}

func TestRecordChannelOperations(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	// Open channel
	m.RecordChannelOpened("opening", "party_a", 100.0, 50.0)
	
	opening := testutil.ToFloat64(m.ChannelsTotal.WithLabelValues("opening"))
	assert.Equal(t, 1.0, opening)
	
	active := testutil.ToFloat64(m.ChannelsActive.WithLabelValues("party_a"))
	assert.Equal(t, 1.0, active)
	
	tvl := testutil.ToFloat64(m.TotalValueLocked.WithLabelValues())
	assert.Equal(t, 150.0, tvl)

	// Activate channel
	m.RecordChannelActivated("success")
	activated := testutil.ToFloat64(m.ChannelOperations.WithLabelValues("activate", "success"))
	assert.Equal(t, 1.0, activated)

	// Close channel
	m.RecordChannelClosed("party_a", "normal", 3600.0, 70.0, 80.0)
	
	closed := testutil.ToFloat64(m.ChannelsTotal.WithLabelValues("closed"))
	assert.Equal(t, 1.0, closed)
	
	active = testutil.ToFloat64(m.ChannelsActive.WithLabelValues("party_a"))
	assert.Equal(t, 0.0, active)
	
	tvl = testutil.ToFloat64(m.TotalValueLocked.WithLabelValues())
	assert.Equal(t, 0.0, tvl)
}

func TestUpdateChannelBalance(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	m.UpdateChannelBalance("channel-001", "party_a", 100.0)
	m.UpdateChannelBalance("channel-001", "party_b", 50.0)
	
	balanceA := testutil.ToFloat64(m.ChannelBalances.WithLabelValues("channel-001", "party_a"))
	assert.Equal(t, 100.0, balanceA)
	
	balanceB := testutil.ToFloat64(m.ChannelBalances.WithLabelValues("channel-001", "party_b"))
	assert.Equal(t, 50.0, balanceB)

	// Update balances after payment
	m.UpdateChannelBalance("channel-001", "party_a", 70.0)
	m.UpdateChannelBalance("channel-001", "party_b", 80.0)
	
	balanceA = testutil.ToFloat64(m.ChannelBalances.WithLabelValues("channel-001", "party_a"))
	assert.Equal(t, 70.0, balanceA)
	
	balanceB = testutil.ToFloat64(m.ChannelBalances.WithLabelValues("channel-001", "party_b"))
	assert.Equal(t, 80.0, balanceB)
}

func TestRecordPayments(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	// Successful payment
	m.RecordPayment("channel-001", 30.0, 1, 0.001, true)
	
	success := testutil.ToFloat64(m.PaymentsTotal.WithLabelValues("success"))
	assert.Equal(t, 1.0, success)
	
	seq := testutil.ToFloat64(m.PaymentSequence.WithLabelValues("channel-001"))
	assert.Equal(t, 1.0, seq)

	// Another successful payment
	m.RecordPayment("channel-001", 20.0, 2, 0.002, true)
	
	success = testutil.ToFloat64(m.PaymentsTotal.WithLabelValues("success"))
	assert.Equal(t, 2.0, success)
	
	seq = testutil.ToFloat64(m.PaymentSequence.WithLabelValues("channel-001"))
	assert.Equal(t, 2.0, seq)

	// Failed payment
	m.RecordPayment("channel-001", 50.0, 3, 0.001, false)
	
	failed := testutil.ToFloat64(m.PaymentsTotal.WithLabelValues("failed"))
	assert.Equal(t, 1.0, failed)
}

func TestRecordPaymentFailure(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	m.RecordPaymentFailure("insufficient_funds")
	m.RecordPaymentFailure("insufficient_funds")
	m.RecordPaymentFailure("channel_expired")
	
	insufficient := testutil.ToFloat64(m.PaymentFailed.WithLabelValues("insufficient_funds"))
	assert.Equal(t, 2.0, insufficient)
	
	expired := testutil.ToFloat64(m.PaymentFailed.WithLabelValues("channel_expired"))
	assert.Equal(t, 1.0, expired)
	
	failed := testutil.ToFloat64(m.PaymentsTotal.WithLabelValues("failed"))
	assert.Equal(t, 3.0, failed)
}

func TestRecordPaymentRefund(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	m.RecordPaymentRefund("channel-001", 25.0)
	
	refunded := testutil.ToFloat64(m.PaymentsTotal.WithLabelValues("refunded"))
	assert.Equal(t, 1.0, refunded)
}

func TestUpdateReputationScore(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	peerID := "peer-001"
	m.UpdateReputationScore(peerID, 0.85, 0.90, 25.5, 1.2)
	
	score := testutil.ToFloat64(m.ReputationScores.WithLabelValues(peerID))
	assert.Equal(t, 0.85, score)
	
	successRate := testutil.ToFloat64(m.SuccessRate.WithLabelValues(peerID))
	assert.Equal(t, 0.90, successRate)
	
	avgDuration := testutil.ToFloat64(m.AverageDuration.WithLabelValues(peerID))
	assert.Equal(t, 25.5, avgDuration)
	
	avgCost := testutil.ToFloat64(m.AverageCost.WithLabelValues(peerID))
	assert.Equal(t, 1.2, avgCost)

	// Update score
	m.UpdateReputationScore(peerID, 0.88, 0.92, 24.0, 1.1)
	
	score = testutil.ToFloat64(m.ReputationScores.WithLabelValues(peerID))
	assert.Equal(t, 0.88, score)
}

func TestRecordTaskExecution(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	peerID := "peer-001"
	
	// Record successes
	m.RecordTaskExecution(peerID, true)
	m.RecordTaskExecution(peerID, true)
	m.RecordTaskExecution(peerID, true)
	
	success := testutil.ToFloat64(m.TasksExecuted.WithLabelValues(peerID, "success"))
	assert.Equal(t, 3.0, success)

	// Record failures
	m.RecordTaskExecution(peerID, false)
	
	failure := testutil.ToFloat64(m.TasksExecuted.WithLabelValues(peerID, "failure"))
	assert.Equal(t, 1.0, failure)
}

func TestRecordBlacklistEvents(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	// Add to blacklist
	m.RecordBlacklistEvent("added")
	m.RecordBlacklistEvent("added")
	
	added := testutil.ToFloat64(m.BlacklistEvents.WithLabelValues("added"))
	assert.Equal(t, 2.0, added)
	
	blacklisted := testutil.ToFloat64(m.BlacklistedPeers.WithLabelValues())
	assert.Equal(t, 2.0, blacklisted)

	// Remove from blacklist
	m.RecordBlacklistEvent("removed")
	
	removed := testutil.ToFloat64(m.BlacklistEvents.WithLabelValues("removed"))
	assert.Equal(t, 1.0, removed)
	
	blacklisted = testutil.ToFloat64(m.BlacklistedPeers.WithLabelValues())
	assert.Equal(t, 1.0, blacklisted)

	// Expire from blacklist
	m.RecordBlacklistEvent("expired")
	
	expired := testutil.ToFloat64(m.BlacklistEvents.WithLabelValues("expired"))
	assert.Equal(t, 1.0, expired)
	
	blacklisted = testutil.ToFloat64(m.BlacklistedPeers.WithLabelValues())
	assert.Equal(t, 0.0, blacklisted)
}

func TestRecordViolations(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	peerID := "peer-001"
	
	m.RecordViolation(peerID, "timeout")
	m.RecordViolation(peerID, "timeout")
	m.RecordViolation(peerID, "failed_task")
	
	timeouts := testutil.ToFloat64(m.ViolationsTotal.WithLabelValues(peerID, "timeout"))
	assert.Equal(t, 2.0, timeouts)
	
	failures := testutil.ToFloat64(m.ViolationsTotal.WithLabelValues(peerID, "failed_task"))
	assert.Equal(t, 1.0, failures)
}

func TestRecordReputationDecay(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	peerID := "peer-001"
	
	m.RecordReputationDecay(peerID)
	m.RecordReputationDecay(peerID)
	
	decay := testutil.ToFloat64(m.ReputationDecay.WithLabelValues(peerID))
	assert.Equal(t, 2.0, decay)
}

func TestRecordSettlements(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	// Successful settlement
	m.RecordSettlement(150.0, 3600.0, true)
	
	success := testutil.ToFloat64(m.SettlementsTotal.WithLabelValues("success"))
	assert.Equal(t, 1.0, success)

	// Failed settlement
	m.RecordSettlement(200.0, 1800.0, false)
	
	failed := testutil.ToFloat64(m.SettlementsTotal.WithLabelValues("failed"))
	assert.Equal(t, 1.0, failed)
}

func TestRecordDisputes(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	m.RecordDispute("resolved", 7200.0)
	m.RecordDispute("pending", 0.0)
	m.RecordDispute("arbitrated", 86400.0)
	
	resolved := testutil.ToFloat64(m.DisputesTotal.WithLabelValues("resolved"))
	assert.Equal(t, 1.0, resolved)
	
	pending := testutil.ToFloat64(m.DisputesTotal.WithLabelValues("pending"))
	assert.Equal(t, 1.0, pending)
	
	arbitrated := testutil.ToFloat64(m.DisputesTotal.WithLabelValues("arbitrated"))
	assert.Equal(t, 1.0, arbitrated)
}

func TestSetActiveParticipants(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	m.SetActiveParticipants(10)
	
	participants := testutil.ToFloat64(m.ActiveParticipants.WithLabelValues())
	assert.Equal(t, 10.0, participants)

	m.SetActiveParticipants(15)
	participants = testutil.ToFloat64(m.ActiveParticipants.WithLabelValues())
	assert.Equal(t, 15.0, participants)
}

func TestRecordTransactionThroughput(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	m.RecordTransactionThroughput(100.0)
	m.RecordTransactionThroughput(150.0)
	m.RecordTransactionThroughput(200.0)
	
	// Histograms just verify they exist
	assert.NotNil(t, m.TransactionThroughput)
}

func TestChannelLifecycle(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	// Complete channel lifecycle
	channelID := "channel-lifecycle-001"
	
	// 1. Open channel
	m.RecordChannelOpened("opening", "party_a", 100.0, 100.0)
	m.RecordChannelOperation("open", "success")
	
	// 2. Activate channel (just record the operation, don't manually adjust state counters)
	m.RecordChannelActivated("success")
	
	// 3. Update balances
	m.UpdateChannelBalance(channelID, "party_a", 100.0)
	m.UpdateChannelBalance(channelID, "party_b", 100.0)
	
	// 4. Make payments
	m.RecordPayment(channelID, 30.0, 1, 0.001, true)
	m.UpdateChannelBalance(channelID, "party_a", 70.0)
	m.UpdateChannelBalance(channelID, "party_b", 130.0)
	
	m.RecordPayment(channelID, 20.0, 2, 0.001, true)
	m.UpdateChannelBalance(channelID, "party_a", 50.0)
	m.UpdateChannelBalance(channelID, "party_b", 150.0)
	
	// 5. Close and settle
	m.RecordChannelClosed("party_a", "normal", 3600.0, 50.0, 150.0)
	m.RecordSettlement(200.0, 3600.0, true)
	
	// Verify final state
	opening := testutil.ToFloat64(m.ChannelsTotal.WithLabelValues("opening"))
	assert.Equal(t, 1.0, opening) // Channel was opened
	
	closed := testutil.ToFloat64(m.ChannelsTotal.WithLabelValues("closed"))
	assert.Equal(t, 1.0, closed) // Channel was closed
	
	payments := testutil.ToFloat64(m.PaymentsTotal.WithLabelValues("success"))
	assert.Equal(t, 2.0, payments)
	
	settlements := testutil.ToFloat64(m.SettlementsTotal.WithLabelValues("success"))
	assert.Equal(t, 1.0, settlements)
}

func TestReputationLifecycle(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	peerID := "peer-reputation-001"
	
	// 1. Execute tasks
	for i := 0; i < 8; i++ {
		m.RecordTaskExecution(peerID, true)
	}
	for i := 0; i < 2; i++ {
		m.RecordTaskExecution(peerID, false)
		m.RecordViolation(peerID, "failed_task")
	}
	
	// 2. Update reputation
	m.UpdateReputationScore(peerID, 0.75, 0.80, 22.0, 1.0)
	
	// 3. Record decay
	m.RecordReputationDecay(peerID)
	
	// 4. Check for blacklist
	score := testutil.ToFloat64(m.ReputationScores.WithLabelValues(peerID))
	assert.Equal(t, 0.75, score)
	
	success := testutil.ToFloat64(m.TasksExecuted.WithLabelValues(peerID, "success"))
	assert.Equal(t, 8.0, success)
	
	failure := testutil.ToFloat64(m.TasksExecuted.WithLabelValues(peerID, "failure"))
	assert.Equal(t, 2.0, failure)
}

func TestEconomicMetricsConcurrency(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	// Concurrent operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			channelID := "concurrent-channel"
			peerID := "concurrent-peer"
			
			for j := 0; j < 100; j++ {
				// Channel operations
				m.RecordPayment(channelID, 10.0, uint64(j), 0.001, true)
				
				// Reputation operations
				m.RecordTaskExecution(peerID, true)
				m.UpdateReputationScore(peerID, 0.85, 0.90, 25.0, 1.0)
			}
			done <- true
		}(i)
	}

	// Wait for completion
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counts
	payments := testutil.ToFloat64(m.PaymentsTotal.WithLabelValues("success"))
	assert.Equal(t, 1000.0, payments)
	
	tasks := testutil.ToFloat64(m.TasksExecuted.WithLabelValues("concurrent-peer", "success"))
	assert.Equal(t, 1000.0, tasks)
}

func BenchmarkRecordPayment(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordPayment("channel-001", 10.0, uint64(i), 0.001, true)
	}
}

func BenchmarkUpdateReputationScore(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.UpdateReputationScore("peer-001", 0.85, 0.90, 25.0, 1.0)
	}
}

func BenchmarkRecordChannelOperation(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewEconomicMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordChannelOperation("open", "success")
	}
}
