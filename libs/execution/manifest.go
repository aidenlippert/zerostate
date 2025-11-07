package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TaskManifest defines the requirements and parameters for a WASM task execution
type TaskManifest struct {
	// Task identification
	TaskID      string    `json:"task_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	
	// Creator information
	CreatorID   peer.ID `json:"creator_id"`
	CreatorSig  []byte  `json:"creator_sig,omitempty"`
	
	// WASM artifact
	ArtifactCID string `json:"artifact_cid"` // IPFS CID of the WASM module
	ArtifactHash string `json:"artifact_hash"` // SHA256 hash for verification
	
	// Resource requirements
	MaxMemory        uint64        `json:"max_memory"`         // Bytes
	MaxExecutionTime time.Duration `json:"max_execution_time"` // Duration
	MaxStackSize     uint64        `json:"max_stack_size,omitempty"`     // Bytes
	
	// Required capabilities (future: GPU, network, etc.)
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	
	// Execution parameters
	FunctionName string   `json:"function_name,omitempty"` // For Execute()
	Args         []string `json:"args,omitempty"`          // For ExecuteWithArgs()
	
	// Input/Output specification
	Inputs  map[string]InputSpec  `json:"inputs,omitempty"`
	Outputs map[string]OutputSpec `json:"outputs,omitempty"`
	
	// Payment terms (future Sprint 4)
	PaymentRequired bool    `json:"payment_required"`
	PricePerSecond  float64 `json:"price_per_second,omitempty"`  // Currency units per second
	PricePerMB      float64 `json:"price_per_mb,omitempty"`      // Currency units per MB
	MaxTotalPrice   float64 `json:"max_total_price,omitempty"`   // Maximum total cost
	
	// Service Level Agreement
	SLA SLA `json:"sla,omitempty"`
}

// InputSpec defines an input parameter for the task
type InputSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`        // "string", "bytes", "int", "float", etc.
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
}

// OutputSpec defines an output from the task
type OutputSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// SLA defines service level agreement terms
type SLA struct {
	MaxStartDelay    time.Duration `json:"max_start_delay,omitempty"`    // Max time to start execution
	RequiredUptime   float64       `json:"required_uptime,omitempty"`    // 0.0-1.0
	MaxFailureRate   float64       `json:"max_failure_rate,omitempty"`   // 0.0-1.0
	MinReputationScore float64     `json:"min_reputation_score,omitempty"` // Minimum executor reputation
}

// Validate checks if the manifest is valid
func (tm *TaskManifest) Validate() error {
	if tm.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if tm.Name == "" {
		return fmt.Errorf("name is required")
	}
	if tm.ArtifactCID == "" {
		return fmt.Errorf("artifact_cid is required")
	}
	if tm.CreatorID == "" {
		return fmt.Errorf("creator_id is required")
	}
	
	// Validate resource limits
	if tm.MaxMemory == 0 {
		return fmt.Errorf("max_memory must be > 0")
	}
	if tm.MaxExecutionTime == 0 {
		return fmt.Errorf("max_execution_time must be > 0")
	}
	
	// Resource limits must not exceed reasonable bounds
	if tm.MaxMemory > 16*1024*1024*1024 { // 16GB max
		return fmt.Errorf("max_memory exceeds limit (16GB)")
	}
	if tm.MaxExecutionTime > 1*time.Hour {
		return fmt.Errorf("max_execution_time exceeds limit (1 hour)")
	}
	
	// Payment validation
	if tm.PaymentRequired {
		if tm.PricePerSecond < 0 || tm.PricePerMB < 0 {
			return fmt.Errorf("prices cannot be negative")
		}
		if tm.MaxTotalPrice < 0 {
			return fmt.Errorf("max_total_price cannot be negative")
		}
	}
	
	// SLA validation
	if tm.SLA.RequiredUptime < 0 || tm.SLA.RequiredUptime > 1.0 {
		return fmt.Errorf("required_uptime must be between 0.0 and 1.0")
	}
	if tm.SLA.MaxFailureRate < 0 || tm.SLA.MaxFailureRate > 1.0 {
		return fmt.Errorf("max_failure_rate must be between 0.0 and 1.0")
	}
	
	return nil
}

// Hash returns the SHA256 hash of the canonical manifest representation
func (tm *TaskManifest) Hash() (string, error) {
	// Create a copy without signature for hashing
	manifestCopy := *tm
	manifestCopy.CreatorSig = nil
	
	data, err := json.Marshal(manifestCopy)
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest: %w", err)
	}
	
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ToJSON serializes the manifest to JSON
func (tm *TaskManifest) ToJSON() ([]byte, error) {
	return json.Marshal(tm)
}

// FromJSON deserializes a manifest from JSON
func FromJSON(data []byte) (*TaskManifest, error) {
	var tm TaskManifest
	if err := json.Unmarshal(data, &tm); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	
	if err := tm.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	
	return &tm, nil
}

// NewTaskManifest creates a new task manifest with defaults
func NewTaskManifest(creatorID peer.ID, name string, artifactCID string) *TaskManifest {
	return &TaskManifest{
		TaskID:           generateTaskID(),
		Name:             name,
		Version:          "1.0.0",
		CreatedAt:        time.Now(),
		CreatorID:        creatorID,
		ArtifactCID:      artifactCID,
		MaxMemory:        DefaultMaxMemory,
		MaxExecutionTime: DefaultMaxExecutionTime,
		MaxStackSize:     DefaultMaxStackSize,
		PaymentRequired:  false,
	}
}

// generateTaskID creates a unique task ID
func generateTaskID() string {
	// Generate from timestamp + random bytes
	data := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 32 character hex string
}

// EstimatePrice calculates the estimated price based on resource usage
func (tm *TaskManifest) EstimatePrice() float64 {
	if !tm.PaymentRequired {
		return 0
	}
	
	// Estimate based on max resource usage
	timeCost := tm.PricePerSecond * tm.MaxExecutionTime.Seconds()
	memoryCostMB := float64(tm.MaxMemory) / (1024 * 1024)
	memoryCost := tm.PricePerMB * memoryCostMB
	
	total := timeCost + memoryCost
	
	// Cap at max price if specified
	if tm.MaxTotalPrice > 0 && total > tm.MaxTotalPrice {
		return tm.MaxTotalPrice
	}
	
	return total
}

// CanExecute checks if the given execution config can satisfy the manifest requirements
func (tm *TaskManifest) CanExecute(config *ExecutionConfig) bool {
	if config.MaxMemory < tm.MaxMemory {
		return false
	}
	if config.MaxExecutionTime < tm.MaxExecutionTime {
		return false
	}
	if config.MaxStackSize < tm.MaxStackSize {
		return false
	}
	if tm.FunctionName != "" && !config.EnableWASI {
		// Function-based execution doesn't strictly require WASI
		// but ExecuteWithArgs does
	}
	
	return true
}
