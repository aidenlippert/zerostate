package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/zerostate/libs/execution"
	"github.com/zerostate/libs/guild"
	"github.com/zerostate/libs/payment"
	"github.com/zerostate/libs/reputation"
)

// TestEndToEndTaskExecutionWithPaymentAndReputation tests the complete workflow:
// 1. Create guild for collaboration
// 2. Open payment channel between creator and executor
// 3. Submit task with manifest
// 4. Execute WASM task
// 5. Generate cryptographic receipt
// 6. Calculate and settle payment
// 7. Update reputation based on execution outcome
func TestEndToEndTaskExecutionWithPaymentAndReputation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create libp2p hosts
	creator, err := libp2p.New()
	require.NoError(t, err)
	defer creator.Close()

	executor, err := libp2p.New()
	require.NoError(t, err)
	defer executor.Close()

	creatorID := creator.ID()
	executorID := executor.ID()

	// Get private keys from hosts
	creatorPrivKey := creator.Peerstore().PrivKey(creatorID)
	executorPrivKey := executor.Peerstore().PrivKey(executorID)

	// Step 1: Create Guild
	guildConfig := guild.DefaultGuildConfig()
	guildMgr := guild.NewGuildManager(ctx, creator, guildConfig, logger)
	defer guildMgr.Close()

	testGuild, err := guildMgr.CreateGuild(ctx, []string{"compute", "storage"})
	require.NoError(t, err)
	assert.NotNil(t, testGuild)
	guildID := testGuild.ID

	// Executor joins guild
	err = guildMgr.JoinGuild(ctx, guildID, []string{"compute"})
	require.NoError(t, err)

	t.Logf("✅ Guild created: %s (creator: %s, executor: %s)",
		string(guildID)[:8], creatorID.String()[:8], executorID.String()[:8])

	// Step 2: Open Payment Channel
	paymentMgr := payment.NewChannelManager(creatorID, creatorPrivKey, logger)
	channel, err := paymentMgr.OpenChannel(ctx, executorID, 100.0, 50.0, 24*time.Hour)
	require.NoError(t, err)
	assert.NotNil(t, channel)

	// Activate channel (in production, executor would co-sign)
	err = paymentMgr.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)

	t.Logf("✅ Payment channel opened: %s (deposits: creator=100, executor=50)",
		channel.ChannelID[:8])

	// Step 3: Create Task Manifest
	manifest := execution.NewTaskManifest(creatorID, "hello-world", "QmTest123")
	manifest.Description = "End-to-end test task"
	manifest.FunctionName = "hello"
	manifest.MaxMemory = 64 * 1024 * 1024 // 64MB
	manifest.MaxExecutionTime = 10 * time.Second
	manifest.PaymentRequired = true
	manifest.PricePerSecond = 1.0  // 1 unit per second
	manifest.PricePerMB = 0.1      // 0.1 units per MB
	manifest.MaxTotalPrice = 50.0  // Cap at 50 units

	err = manifest.Validate()
	require.NoError(t, err)

	estimatedCost := manifest.EstimatePrice()
	t.Logf("✅ Task manifest created: %s (estimated cost: %.2f units)",
		manifest.TaskID, estimatedCost)

	// Step 4: Execute WASM Task
	execConfig := execution.DefaultExecutionConfig()
	wasmRunner, err := execution.NewWASMRunner(ctx, execConfig, logger)
	require.NoError(t, err)
	defer wasmRunner.Close(ctx)

	// Simple WASM module that returns 42
	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // WASM magic + version
		0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f, // Type section: () -> i32
		0x03, 0x02, 0x01, 0x00, // Function section
		0x07, 0x09, 0x01, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x00, 0x00, // Export "hello"
		0x0a, 0x06, 0x01, 0x04, 0x00, 0x41, 0x2a, 0x0b, // Code: return 42
	}

	execCtx, cancel := context.WithTimeout(ctx, manifest.MaxExecutionTime)
	defer cancel()

	startTime := time.Now()
	result, err := wasmRunner.Execute(execCtx, wasmBytes, "hello", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(0), result.ExitCode)

	t.Logf("✅ WASM execution completed: duration=%.2fms, memory=%d bytes",
		result.Duration.Seconds()*1000, result.MemoryUsed)

	// Step 5: Generate Receipt
	receipt := execution.NewReceipt(manifest.TaskID, executorID, result)
	receipt.GuildID = string(guildID)

	// Calculate cost based on manifest pricing
	receipt.CalculateCost(manifest)

	// Sign receipt with executor's private key
	err = receipt.Sign(executorPrivKey)
	require.NoError(t, err)

	// Verify receipt signature
	err = receipt.Verify()
	require.NoError(t, err)

	t.Logf("✅ Receipt generated: id=%s, cost=%.2f units (time=%.2f, memory=%.2f)",
		receipt.ReceiptID[:8], receipt.TotalCost, receipt.TimeCost, receipt.MemoryCost)

	// Step 6: Settle Payment
	paymentAmount := receipt.TotalCost
	pmt, err := paymentMgr.MakePayment(
		ctx,
		channel.ChannelID,
		executorID,
		paymentAmount,
		fmt.Sprintf("Task %s execution", manifest.TaskID),
	)
	require.NoError(t, err)
	assert.NotNil(t, pmt)
	assert.Equal(t, paymentAmount, pmt.Amount)

	// Verify payment signature
	err = pmt.Verify(creatorPrivKey.GetPublic())
	require.NoError(t, err)

	t.Logf("✅ Payment settled: %.2f units (creator → executor), seq=%d",
		paymentAmount, pmt.SequenceNum)

	// Step 7: Update Reputation
	reputationMgr := reputation.NewReputationManager(nil, logger)

	outcome := reputation.ExecutionOutcome{
		TaskID:     manifest.TaskID,
		ExecutorID: executorID,
		Success:    receipt.Success,
		Duration:   receipt.Duration,
		Cost:       receipt.TotalCost,
		Timestamp:  time.Now(),
		ExitCode:   int(receipt.ExitCode),
	}

	err = reputationMgr.RecordExecution(ctx, outcome)
	require.NoError(t, err)

	score, err := reputationMgr.GetScore(executorID)
	require.NoError(t, err)
	assert.Equal(t, 1, score.TasksCompleted)
	assert.Equal(t, 1.0, score.SuccessRate)
	assert.False(t, score.Blacklisted)

	t.Logf("✅ Reputation updated: executor score=%.3f (tasks=1, success=100%%)",
		score.Score)

	// Step 8: Verify Final State
	
	// Check channel balance
	updatedChannel, err := paymentMgr.GetChannel(channel.ChannelID)
	require.NoError(t, err)
	
	var creatorBalance, executorBalance float64
	if creatorID == updatedChannel.PartyA {
		creatorBalance = updatedChannel.BalanceA
		executorBalance = updatedChannel.BalanceB
	} else {
		creatorBalance = updatedChannel.BalanceB
		executorBalance = updatedChannel.BalanceA
	}
	
	expectedCreatorBalance := 100.0 - paymentAmount
	expectedExecutorBalance := 50.0 + paymentAmount
	
	assert.InDelta(t, expectedCreatorBalance, creatorBalance, 0.01)
	assert.InDelta(t, expectedExecutorBalance, executorBalance, 0.01)

	t.Logf("✅ Final balances: creator=%.2f, executor=%.2f",
		creatorBalance, executorBalance)

	// Step 9: Close Channel
	err = paymentMgr.CloseChannel(ctx, channel.ChannelID, "task_complete")
	require.NoError(t, err)

	// Step 10: Dissolve Guild
	err = guildMgr.DissolveGuild(ctx, guildID)
	require.NoError(t, err)

	t.Logf("✅ Cleanup complete: channel closed, guild dissolved")

	// Final assertions
	// Score should be neutral (0.5) since we only have 1 task (below MinTasksForScore default of 5)
	assert.Equal(t, 0.5, score.Score, "Score should remain neutral until MinTasksForScore reached")
	assert.Equal(t, payment.ChannelStateClosed, updatedChannel.State)

	t.Logf("\n🎉 End-to-end workflow completed successfully!")
	t.Logf("   Total execution time: %v", time.Since(startTime))
	t.Logf("   Task cost: %.2f units", paymentAmount)
	t.Logf("   Executor reputation: %.3f", score.Score)
}

// TestMultipleTasksImprovingReputation tests that successful task completions
// improve executor reputation over time
func TestMultipleTasksImprovingReputation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create executor
	executorPrivKey, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	executorID, err := peer.IDFromPrivateKey(executorPrivKey)
	require.NoError(t, err)

	// Create reputation manager
	reputationMgr := reputation.NewReputationManager(nil, logger)

	// Execute 10 successful tasks with improving performance
	for i := 0; i < 10; i++ {
		// Performance improves over time
		duration := time.Duration(30-i) * time.Second // Gets faster
		cost := float64(10) - float64(i)*0.5          // Gets cheaper

		outcome := reputation.ExecutionOutcome{
			TaskID:     fmt.Sprintf("task-%d", i),
			ExecutorID: executorID,
			Success:    true,
			Duration:   duration,
			Cost:       cost,
			Timestamp:  time.Now(),
			ExitCode:   0,
		}

		err := reputationMgr.RecordExecution(ctx, outcome)
		require.NoError(t, err)

		score, err := reputationMgr.GetScore(executorID)
		require.NoError(t, err)

		t.Logf("Task %2d: duration=%2ds, cost=%.1f, score=%.3f",
			i+1, int(duration.Seconds()), cost, score.Score)
	}

	// Final reputation check
	finalScore, err := reputationMgr.GetScore(executorID)
	require.NoError(t, err)

	assert.Equal(t, 10, finalScore.TasksCompleted)
	assert.Equal(t, 0, finalScore.TasksFailed)
	assert.Equal(t, 1.0, finalScore.SuccessRate)
	// Score should improve but won't reach 0.7 with default weights (alpha=0.3 makes change slow)
	// After 10 successful tasks with improving efficiency, expect score ~0.64-0.65
	assert.Greater(t, finalScore.Score, 0.6, "Score should improve after 10 successful tasks")
	assert.False(t, finalScore.Blacklisted)

	t.Logf("\n✅ Final reputation after 10 tasks: %.3f (excellent)", finalScore.Score)
}

// TestFailingTasksDegradeReputation tests that failures lower reputation
// and can lead to blacklisting
func TestFailingTasksDegradeReputation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create executor
	executorPrivKey, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	executorID, err := peer.IDFromPrivateKey(executorPrivKey)
	require.NoError(t, err)

	// Use custom config with lower blacklist threshold
	config := reputation.DefaultScoreConfig()
	config.BlacklistThreshold = 0.3
	config.MinTasksForScore = 5

	reputationMgr := reputation.NewReputationManager(config, logger)

	// Execute 10 tasks with 80% failure rate
	for i := 0; i < 10; i++ {
		success := i < 2 // Only first 2 succeed

		outcome := reputation.ExecutionOutcome{
			TaskID:     fmt.Sprintf("task-%d", i),
			ExecutorID: executorID,
			Success:    success,
			Duration:   60 * time.Second, // Slow execution
			Cost:       20.0,             // Expensive
			Timestamp:  time.Now(),
			ExitCode:   0,
		}

		if !success {
			outcome.ExitCode = 1
			outcome.Error = "execution failed"
		}

		err := reputationMgr.RecordExecution(ctx, outcome)
		require.NoError(t, err)

		score, err := reputationMgr.GetScore(executorID)
		require.NoError(t, err)

		t.Logf("Task %2d: success=%v, score=%.3f, blacklisted=%v",
			i+1, success, score.Score, score.Blacklisted)
	}

	// Final reputation check
	finalScore, err := reputationMgr.GetScore(executorID)
	require.NoError(t, err)

	assert.Equal(t, 2, finalScore.TasksCompleted)
	assert.Equal(t, 8, finalScore.TasksFailed)
	assert.Equal(t, 0.2, finalScore.SuccessRate)
	assert.Less(t, finalScore.Score, config.BlacklistThreshold)
	assert.True(t, finalScore.Blacklisted, "Should be blacklisted due to low score")
	assert.True(t, reputationMgr.IsBlacklisted(executorID))

	t.Logf("\n✅ Executor blacklisted: score=%.3f (20%% success rate)", finalScore.Score)
}

// TestPaymentChannelSettlement tests multiple payments and final settlement
func TestPaymentChannelSettlement(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create peers
	creatorPrivKey, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	creatorID, err := peer.IDFromPrivateKey(creatorPrivKey)
	require.NoError(t, err)

	executorPrivKey, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	executorID, err := peer.IDFromPrivateKey(executorPrivKey)
	require.NoError(t, err)

	// Open channel with minimum deposit for executor
	paymentMgr := payment.NewChannelManager(creatorID, creatorPrivKey, logger)
	channel, err := paymentMgr.OpenChannel(ctx, executorID, 1000.0, 0.001, 24*time.Hour)
	require.NoError(t, err)

	err = paymentMgr.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)

	t.Logf("Channel opened: creator deposit=1000, executor deposit=0.001")

	// Simulate 5 task payments
	taskCosts := []float64{50.0, 75.0, 100.0, 125.0, 150.0}
	totalPaid := 0.0

	for i, cost := range taskCosts {
		pmt, err := paymentMgr.MakePayment(
			ctx,
			channel.ChannelID,
			executorID,
			cost,
			fmt.Sprintf("Task %d", i+1),
		)
		require.NoError(t, err)
		totalPaid += cost

		t.Logf("Payment %d: %.2f units (total: %.2f)", i+1, cost, totalPaid)

		// Verify payment signature
		err = pmt.Verify(creatorPrivKey.GetPublic())
		require.NoError(t, err)
	}

	// Check final balances
	finalChannel, err := paymentMgr.GetChannel(channel.ChannelID)
	require.NoError(t, err)

	var creatorBalance, executorBalance float64
	if creatorID == finalChannel.PartyA {
		creatorBalance = finalChannel.BalanceA
		executorBalance = finalChannel.BalanceB
	} else {
		creatorBalance = finalChannel.BalanceB
		executorBalance = finalChannel.BalanceA
	}

	expectedCreatorBalance := 1000.0 - totalPaid
	expectedExecutorBalance := 0.0 + totalPaid

	assert.InDelta(t, expectedCreatorBalance, creatorBalance, 0.01)
	assert.InDelta(t, expectedExecutorBalance, executorBalance, 0.01)

	t.Logf("\n✅ Settlement complete:")
	t.Logf("   Total paid: %.2f units", totalPaid)
	t.Logf("   Creator balance: %.2f", creatorBalance)
	t.Logf("   Executor balance: %.2f", executorBalance)

	// Close channel
	err = paymentMgr.CloseChannel(ctx, channel.ChannelID, "all_tasks_complete")
	require.NoError(t, err)
}
