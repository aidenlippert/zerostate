package execution

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReceipt(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	result := &ExecutionResult{
		ExitCode:   0,
		Output:     []byte("test output"),
		Error:      nil,
		Duration:   100 * time.Millisecond,
		MemoryUsed: 1024 * 1024, // 1MB
		GasUsed:    500,
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(100 * time.Millisecond),
	}
	
	receipt := NewReceipt("task-123", host.ID(), result)
	
	assert.NotEmpty(t, receipt.ReceiptID)
	assert.Equal(t, "task-123", receipt.TaskID)
	assert.Equal(t, host.ID(), receipt.ExecutorID)
	assert.True(t, receipt.Success)
	assert.Equal(t, int32(0), receipt.ExitCode)
	assert.Equal(t, result.Duration, receipt.Duration)
	assert.Equal(t, result.MemoryUsed, receipt.MemoryUsed)
	assert.Equal(t, result.Output, receipt.Output)
	assert.NotEmpty(t, receipt.OutputHash)
	assert.Empty(t, receipt.Error)
}

func TestNewReceiptWithError(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	result := &ExecutionResult{
		ExitCode:  1,
		Error:     ErrTimeout,
		Duration:  30 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(30 * time.Second),
	}
	
	receipt := NewReceipt("task-456", host.ID(), result)
	
	assert.False(t, receipt.Success)
	assert.Equal(t, int32(1), receipt.ExitCode)
	assert.Equal(t, ErrTimeout.Error(), receipt.Error)
	assert.Empty(t, receipt.Output)
	assert.Empty(t, receipt.OutputHash)
}

func TestReceiptCalculateCost(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	result := &ExecutionResult{
		ExitCode:   0,
		Duration:   10 * time.Second,
		MemoryUsed: 100 * 1024 * 1024, // 100MB
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(10 * time.Second),
	}
	
	receipt := NewReceipt("task-789", host.ID(), result)
	
	manifest := NewTaskManifest(host.ID(), "test", "QmTest")
	manifest.PaymentRequired = true
	manifest.PricePerSecond = 0.001  // $0.001/sec
	manifest.PricePerMB = 0.0001     // $0.0001/MB
	
	receipt.CalculateCost(manifest)
	
	expectedTime := 0.001 * 10      // $0.01
	expectedMem := 0.0001 * 100     // $0.01
	expectedTotal := expectedTime + expectedMem // $0.02
	
	assert.InDelta(t, expectedTime, receipt.TimeCost, 0.001)
	assert.InDelta(t, expectedMem, receipt.MemoryCost, 0.001)
	assert.InDelta(t, expectedTotal, receipt.TotalCost, 0.001)
}

func TestReceiptCalculateCostWithCap(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	result := &ExecutionResult{
		ExitCode:   0,
		Duration:   100 * time.Second,
		MemoryUsed: 1000 * 1024 * 1024, // 1000MB = 1GB
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(100 * time.Second),
	}
	
	receipt := NewReceipt("task-cap", host.ID(), result)
	
	manifest := NewTaskManifest(host.ID(), "test", "QmTest")
	manifest.PaymentRequired = true
	manifest.PricePerSecond = 0.001
	manifest.PricePerMB = 0.0001
	manifest.MaxTotalPrice = 0.05 // Cap at $0.05
	
	receipt.CalculateCost(manifest)
	
	// Would be: 0.1 + 0.1 = 0.2, but capped at 0.05
	assert.Equal(t, 0.05, receipt.TotalCost)
}

func TestReceiptHash(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	result := &ExecutionResult{
		ExitCode:  0,
		Duration:  1 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
	}
	
	receipt := NewReceipt("task-hash", host.ID(), result)
	
	hash1, err := receipt.Hash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA256 hex
	
	// Same receipt should produce same hash
	hash2, err := receipt.Hash()
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2)
	
	// Hash should be stable even with signature
	receipt.ExecutorSig = []byte("fake-sig")
	hash3, err := receipt.Hash()
	require.NoError(t, err)
	assert.Equal(t, hash1, hash3)
	
	// Different receipt should produce different hash
	result2 := &ExecutionResult{
		ExitCode:  1,
		Duration:  2 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(2 * time.Second),
	}
	receipt2 := NewReceipt("task-hash-2", host.ID(), result2)
	hash4, err := receipt2.Hash()
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash4)
}

func TestReceiptSignAndVerify(t *testing.T) {
	// Create host with deterministic key
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	
	executorID, err := peer.IDFromPublicKey(pub)
	require.NoError(t, err)
	
	result := &ExecutionResult{
		ExitCode:  0,
		Duration:  1 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
	}
	
	receipt := NewReceipt("task-sig", executorID, result)
	
	// Sign the receipt
	err = receipt.Sign(priv)
	require.NoError(t, err)
	assert.NotEmpty(t, receipt.ExecutorSig)
	
	// Verify the signature
	err = receipt.Verify()
	assert.NoError(t, err)
	
	// Tamper with receipt
	receipt.ExitCode = 1
	err = receipt.Verify()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestReceiptAttestation(t *testing.T) {
	// Create executor
	executorPriv, executorPub, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	executorID, err := peer.IDFromPublicKey(executorPub)
	require.NoError(t, err)
	
	// Create witness
	witnessPriv, witnessPub, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	witnessID, err := peer.IDFromPublicKey(witnessPub)
	require.NoError(t, err)
	
	result := &ExecutionResult{
		ExitCode:  0,
		Duration:  1 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
	}
	
	receipt := NewReceipt("task-att", executorID, result)
	
	// Sign receipt
	err = receipt.Sign(executorPriv)
	require.NoError(t, err)
	
	// Get receipt hash for witness to sign
	hash, err := receipt.Hash()
	require.NoError(t, err)
	
	// Witness signs the hash
	witnessSig, err := witnessPriv.Sign([]byte(hash))
	require.NoError(t, err)
	
	// Add attestation
	receipt.AddAttestation(witnessID, witnessSig)
	
	assert.Equal(t, 1, receipt.GetAttestationCount())
	assert.True(t, receipt.HasAttestation(witnessID))
	
	// Verify attestation
	err = receipt.VerifyAttestation(0)
	assert.NoError(t, err)
}

func TestReceiptJSON(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	result := &ExecutionResult{
		ExitCode:   0,
		Output:     []byte("test output"),
		Duration:   5 * time.Second,
		MemoryUsed: 2 * 1024 * 1024, // 2MB
		GasUsed:    1000,
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(5 * time.Second),
	}
	
	receipt := NewReceipt("task-json", host.ID(), result)
	receipt.GuildID = "guild-123"
	receipt.TimeCost = 0.005
	receipt.MemoryCost = 0.002
	receipt.TotalCost = 0.007
	
	// Sign it
	err = receipt.Sign(host.Peerstore().PrivKey(host.ID()))
	require.NoError(t, err)
	
	// Serialize
	data, err := receipt.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	
	// Deserialize
	receipt2, err := ReceiptFromJSON(data)
	require.NoError(t, err)
	assert.Equal(t, receipt.ReceiptID, receipt2.ReceiptID)
	assert.Equal(t, receipt.TaskID, receipt2.TaskID)
	assert.Equal(t, receipt.GuildID, receipt2.GuildID)
	assert.Equal(t, receipt.ExecutorID, receipt2.ExecutorID)
	assert.Equal(t, receipt.Success, receipt2.Success)
	assert.Equal(t, receipt.ExitCode, receipt2.ExitCode)
	assert.Equal(t, receipt.Duration, receipt2.Duration)
	assert.Equal(t, receipt.MemoryUsed, receipt2.MemoryUsed)
	assert.Equal(t, receipt.TotalCost, receipt2.TotalCost)
	assert.Equal(t, receipt.ExecutorSig, receipt2.ExecutorSig)
	
	// Verify signature after deserialization
	err = receipt2.Verify()
	assert.NoError(t, err)
}

func TestReceiptValidate(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	result := &ExecutionResult{
		ExitCode:  0,
		Duration:  1 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
	}
	
	tests := []struct {
		name    string
		modify  func(*Receipt)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid receipt",
			modify:  func(r *Receipt) {},
			wantErr: false,
		},
		{
			name:    "missing receipt_id",
			modify:  func(r *Receipt) { r.ReceiptID = "" },
			wantErr: true,
			errMsg:  "receipt_id is required",
		},
		{
			name:    "missing task_id",
			modify:  func(r *Receipt) { r.TaskID = "" },
			wantErr: true,
			errMsg:  "task_id is required",
		},
		{
			name:    "zero start_time",
			modify:  func(r *Receipt) { r.StartTime = time.Time{} },
			wantErr: true,
			errMsg:  "start_time is required",
		},
		{
			name:    "zero end_time",
			modify:  func(r *Receipt) { r.EndTime = time.Time{} },
			wantErr: true,
			errMsg:  "end_time is required",
		},
		{
			name: "end_time before start_time",
			modify: func(r *Receipt) {
				r.StartTime = time.Now()
				r.EndTime = time.Now().Add(-1 * time.Second)
			},
			wantErr: true,
			errMsg:  "end_time must be after start_time",
		},
		{
			name:    "zero duration",
			modify:  func(r *Receipt) { r.Duration = 0 },
			wantErr: true,
			errMsg:  "duration must be > 0",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receipt := NewReceipt("task-validate", host.ID(), result)
			tt.modify(receipt)
			
			err := receipt.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReceiptIsSuccessful(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	// Successful execution
	result1 := &ExecutionResult{
		ExitCode:  0,
		Error:     nil,
		Duration:  1 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
	}
	receipt1 := NewReceipt("task-success", host.ID(), result1)
	assert.True(t, receipt1.IsSuccessful())
	
	// Failed execution (non-zero exit code)
	result2 := &ExecutionResult{
		ExitCode:  1,
		Duration:  1 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
	}
	receipt2 := NewReceipt("task-fail", host.ID(), result2)
	assert.False(t, receipt2.IsSuccessful())
	
	// Failed execution (with error)
	result3 := &ExecutionResult{
		ExitCode:  0,
		Error:     ErrTimeout,
		Duration:  30 * time.Second,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(30 * time.Second),
	}
	receipt3 := NewReceipt("task-timeout", host.ID(), result3)
	assert.False(t, receipt3.IsSuccessful())
}
