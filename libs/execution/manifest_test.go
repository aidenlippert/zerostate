package execution

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskManifest(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	manifest := NewTaskManifest(host.ID(), "test-task", "QmTest123")
	
	assert.NotEmpty(t, manifest.TaskID)
	assert.Equal(t, "test-task", manifest.Name)
	assert.Equal(t, "QmTest123", manifest.ArtifactCID)
	assert.Equal(t, host.ID(), manifest.CreatorID)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.Equal(t, uint64(DefaultMaxMemory), manifest.MaxMemory)
	assert.Equal(t, DefaultMaxExecutionTime, manifest.MaxExecutionTime)
	assert.False(t, manifest.PaymentRequired)
}

func TestTaskManifestValidate(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	tests := []struct {
		name    string
		modify  func(*TaskManifest)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid manifest",
			modify:  func(tm *TaskManifest) {},
			wantErr: false,
		},
		{
			name:    "missing task_id",
			modify:  func(tm *TaskManifest) { tm.TaskID = "" },
			wantErr: true,
			errMsg:  "task_id is required",
		},
		{
			name:    "missing name",
			modify:  func(tm *TaskManifest) { tm.Name = "" },
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "missing artifact_cid",
			modify:  func(tm *TaskManifest) { tm.ArtifactCID = "" },
			wantErr: true,
			errMsg:  "artifact_cid is required",
		},
		{
			name:    "zero max_memory",
			modify:  func(tm *TaskManifest) { tm.MaxMemory = 0 },
			wantErr: true,
			errMsg:  "max_memory must be > 0",
		},
		{
			name:    "zero max_execution_time",
			modify:  func(tm *TaskManifest) { tm.MaxExecutionTime = 0 },
			wantErr: true,
			errMsg:  "max_execution_time must be > 0",
		},
		{
			name:    "excessive max_memory",
			modify:  func(tm *TaskManifest) { tm.MaxMemory = 20 * 1024 * 1024 * 1024 }, // 20GB
			wantErr: true,
			errMsg:  "max_memory exceeds limit",
		},
		{
			name:    "excessive max_execution_time",
			modify:  func(tm *TaskManifest) { tm.MaxExecutionTime = 2 * time.Hour },
			wantErr: true,
			errMsg:  "max_execution_time exceeds limit",
		},
		{
			name: "negative price",
			modify: func(tm *TaskManifest) {
				tm.PaymentRequired = true
				tm.PricePerSecond = -1.0
			},
			wantErr: true,
			errMsg:  "prices cannot be negative",
		},
		{
			name: "invalid uptime",
			modify: func(tm *TaskManifest) {
				tm.SLA.RequiredUptime = 1.5
			},
			wantErr: true,
			errMsg:  "required_uptime must be between",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := NewTaskManifest(host.ID(), "test", "QmTest")
			tt.modify(manifest)
			
			err := manifest.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManifestHash(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	manifest := NewTaskManifest(host.ID(), "test", "QmTest")
	
	hash1, err := manifest.Hash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA256 hex string
	
	// Same manifest should produce same hash
	hash2, err := manifest.Hash()
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2)
	
	// Different manifest should produce different hash
	manifest2 := NewTaskManifest(host.ID(), "test2", "QmTest")
	hash3, err := manifest2.Hash()
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash3)
	
	// Hash should be stable even with signature
	manifest.CreatorSig = []byte("fake-signature")
	hash4, err := manifest.Hash()
	require.NoError(t, err)
	assert.Equal(t, hash1, hash4) // Signature should not affect hash
}

func TestTaskManifestJSON(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	manifest := NewTaskManifest(host.ID(), "test-task", "QmTest123")
	manifest.Description = "Test task description"
	manifest.FunctionName = "main"
	manifest.Args = []string{"arg1", "arg2"}
	manifest.PaymentRequired = true
	manifest.PricePerSecond = 0.001
	manifest.PricePerMB = 0.0001
	
	// Serialize to JSON
	data, err := manifest.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	
	// Deserialize from JSON
	manifest2, err := FromJSON(data)
	require.NoError(t, err)
	assert.Equal(t, manifest.TaskID, manifest2.TaskID)
	assert.Equal(t, manifest.Name, manifest2.Name)
	assert.Equal(t, manifest.Description, manifest2.Description)
	assert.Equal(t, manifest.FunctionName, manifest2.FunctionName)
	assert.Equal(t, manifest.Args, manifest2.Args)
	assert.Equal(t, manifest.PaymentRequired, manifest2.PaymentRequired)
	assert.Equal(t, manifest.PricePerSecond, manifest2.PricePerSecond)
}

func TestTaskManifestEstimatePrice(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	manifest := NewTaskManifest(host.ID(), "test", "QmTest")
	
	// No payment required
	price := manifest.EstimatePrice()
	assert.Equal(t, 0.0, price)
	
	// With payment
	manifest.PaymentRequired = true
	manifest.PricePerSecond = 0.001  // $0.001 per second
	manifest.PricePerMB = 0.0001     // $0.0001 per MB
	manifest.MaxExecutionTime = 10 * time.Second
	manifest.MaxMemory = 100 * 1024 * 1024 // 100MB
	
	price = manifest.EstimatePrice()
	expectedTime := 0.001 * 10        // $0.01
	expectedMem := 0.0001 * 100       // $0.01
	expected := expectedTime + expectedMem // $0.02
	assert.InDelta(t, expected, price, 0.001)
	
	// With max price cap
	manifest.MaxTotalPrice = 0.015
	price = manifest.EstimatePrice()
	assert.Equal(t, 0.015, price) // Capped at max
}

func TestTaskManifestCanExecute(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	manifest := NewTaskManifest(host.ID(), "test", "QmTest")
	manifest.MaxMemory = 128 * 1024 * 1024       // 128MB
	manifest.MaxExecutionTime = 30 * time.Second
	manifest.MaxStackSize = 8 * 1024 * 1024      // 8MB
	
	// Config that can execute
	config := DefaultExecutionConfig()
	assert.True(t, manifest.CanExecute(config))
	
	// Insufficient memory
	configLowMem := DefaultExecutionConfig()
	configLowMem.MaxMemory = 64 * 1024 * 1024 // 64MB
	assert.False(t, manifest.CanExecute(configLowMem))
	
	// Insufficient time
	configLowTime := DefaultExecutionConfig()
	configLowTime.MaxExecutionTime = 10 * time.Second
	assert.False(t, manifest.CanExecute(configLowTime))
	
	// Insufficient stack
	configLowStack := DefaultExecutionConfig()
	configLowStack.MaxStackSize = 4 * 1024 * 1024 // 4MB
	assert.False(t, manifest.CanExecute(configLowStack))
}

func TestTaskManifestInputOutput(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	manifest := NewTaskManifest(host.ID(), "test", "QmTest")
	
	// Add inputs
	manifest.Inputs = map[string]InputSpec{
		"data": {
			Name:        "data",
			Type:        "bytes",
			Required:    true,
			Description: "Input data to process",
		},
		"threshold": {
			Name:         "threshold",
			Type:         "float",
			Required:     false,
			DefaultValue: "0.5",
			Description:  "Processing threshold",
		},
	}
	
	// Add outputs
	manifest.Outputs = map[string]OutputSpec{
		"result": {
			Name:        "result",
			Type:        "bytes",
			Description: "Processed output",
		},
		"confidence": {
			Name:        "confidence",
			Type:        "float",
			Description: "Confidence score",
		},
	}
	
	// Validate
	err = manifest.Validate()
	assert.NoError(t, err)
	
	// Serialize and deserialize
	data, err := manifest.ToJSON()
	require.NoError(t, err)
	
	manifest2, err := FromJSON(data)
	require.NoError(t, err)
	
	assert.Len(t, manifest2.Inputs, 2)
	assert.Len(t, manifest2.Outputs, 2)
	assert.Equal(t, "bytes", manifest2.Inputs["data"].Type)
	assert.True(t, manifest2.Inputs["data"].Required)
	assert.Equal(t, "0.5", manifest2.Inputs["threshold"].DefaultValue)
}

func TestTaskManifestSLA(t *testing.T) {
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()
	
	manifest := NewTaskManifest(host.ID(), "test", "QmTest")
	manifest.SLA = SLA{
		MaxStartDelay:      5 * time.Second,
		RequiredUptime:     0.99,
		MaxFailureRate:     0.01,
		MinReputationScore: 0.8,
	}
	
	err = manifest.Validate()
	assert.NoError(t, err)
	
	// Invalid SLA
	manifest.SLA.RequiredUptime = 1.5
	err = manifest.Validate()
	assert.Error(t, err)
}
