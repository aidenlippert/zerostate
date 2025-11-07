package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Receipt is a signed proof of task execution with resource usage
type Receipt struct {
	// Task and execution identification
	ReceiptID   string    `json:"receipt_id"`
	TaskID      string    `json:"task_id"`
	GuildID     string    `json:"guild_id,omitempty"` // Optional: which guild executed
	ExecutorID  peer.ID   `json:"executor_id"`
	CreatedAt   time.Time `json:"created_at"`
	
	// Execution results
	Success     bool      `json:"success"`
	ExitCode    int32     `json:"exit_code"`
	Error       string    `json:"error,omitempty"`
	
	// Resource usage (actual)
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	MemoryUsed  uint64        `json:"memory_used"`  // Bytes
	GasUsed     uint64        `json:"gas_used,omitempty"`     // Future: for metering
	
	// Output data
	Output      []byte    `json:"output,omitempty"`       // Actual output from execution
	OutputHash  string    `json:"output_hash,omitempty"`  // SHA256 of output for verification
	
	// Payment calculation (based on actual usage)
	TimeCost    float64   `json:"time_cost,omitempty"`
	MemoryCost  float64   `json:"memory_cost,omitempty"`
	TotalCost   float64   `json:"total_cost,omitempty"`
	
	// Cryptographic proof
	ExecutorSig []byte    `json:"executor_sig"` // Signature over receipt hash
	
	// Attestations (future: multi-party verification)
	Attestations []Attestation `json:"attestations,omitempty"`
}

// Attestation is a witness signature verifying the receipt
type Attestation struct {
	WitnessID   peer.ID   `json:"witness_id"`
	WitnessSig  []byte    `json:"witness_sig"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewReceipt creates a new receipt from an execution result
func NewReceipt(taskID string, executorID peer.ID, result *ExecutionResult) *Receipt {
	receipt := &Receipt{
		ReceiptID:  generateReceiptID(),
		TaskID:     taskID,
		ExecutorID: executorID,
		CreatedAt:  time.Now(),
		Success:    result.Error == nil && result.ExitCode == 0,
		ExitCode:   result.ExitCode,
		StartTime:  result.StartTime,
		EndTime:    result.EndTime,
		Duration:   result.Duration,
		MemoryUsed: result.MemoryUsed,
		GasUsed:    result.GasUsed,
		Output:     result.Output,
	}
	
	if result.Error != nil {
		receipt.Error = result.Error.Error()
	}
	
	// Generate output hash if there's output
	if len(result.Output) > 0 {
		hash := sha256.Sum256(result.Output)
		receipt.OutputHash = hex.EncodeToString(hash[:])
	}
	
	return receipt
}

// CalculateCost computes the cost based on manifest pricing
func (r *Receipt) CalculateCost(manifest *TaskManifest) {
	if !manifest.PaymentRequired {
		return
	}
	
	// Calculate actual costs based on usage
	r.TimeCost = manifest.PricePerSecond * r.Duration.Seconds()
	
	memoryMB := float64(r.MemoryUsed) / (1024 * 1024)
	r.MemoryCost = manifest.PricePerMB * memoryMB
	
	r.TotalCost = r.TimeCost + r.MemoryCost
	
	// Cap at max price if specified
	if manifest.MaxTotalPrice > 0 && r.TotalCost > manifest.MaxTotalPrice {
		r.TotalCost = manifest.MaxTotalPrice
	}
}

// Hash returns the SHA256 hash of the canonical receipt representation
func (r *Receipt) Hash() (string, error) {
	// Create copy without signatures for hashing
	receiptCopy := *r
	receiptCopy.ExecutorSig = nil
	receiptCopy.Attestations = nil
	
	data, err := json.Marshal(receiptCopy)
	if err != nil {
		return "", fmt.Errorf("failed to marshal receipt: %w", err)
	}
	
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// Sign signs the receipt with the executor's private key
func (r *Receipt) Sign(privKey crypto.PrivKey) error {
	hash, err := r.Hash()
	if err != nil {
		return fmt.Errorf("failed to hash receipt: %w", err)
	}
	
	sig, err := privKey.Sign([]byte(hash))
	if err != nil {
		return fmt.Errorf("failed to sign receipt: %w", err)
	}
	
	r.ExecutorSig = sig
	return nil
}

// Verify verifies the executor's signature on the receipt
func (r *Receipt) Verify() error {
	if len(r.ExecutorSig) == 0 {
		return fmt.Errorf("receipt is not signed")
	}
	
	// Extract public key from executor peer ID
	pubKey, err := r.ExecutorID.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("failed to extract public key: %w", err)
	}
	
	hash, err := r.Hash()
	if err != nil {
		return fmt.Errorf("failed to hash receipt: %w", err)
	}
	
	valid, err := pubKey.Verify([]byte(hash), r.ExecutorSig)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}
	
	if !valid {
		return fmt.Errorf("invalid signature")
	}
	
	return nil
}

// AddAttestation adds a witness attestation to the receipt
func (r *Receipt) AddAttestation(witnessID peer.ID, witnessSig []byte) {
	attestation := Attestation{
		WitnessID:  witnessID,
		WitnessSig: witnessSig,
		Timestamp:  time.Now(),
	}
	r.Attestations = append(r.Attestations, attestation)
}

// VerifyAttestation verifies a specific attestation
func (r *Receipt) VerifyAttestation(idx int) error {
	if idx < 0 || idx >= len(r.Attestations) {
		return fmt.Errorf("attestation index out of range")
	}
	
	attestation := r.Attestations[idx]
	
	// Extract witness public key
	pubKey, err := attestation.WitnessID.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("failed to extract witness public key: %w", err)
	}
	
	// Get receipt hash
	hash, err := r.Hash()
	if err != nil {
		return fmt.Errorf("failed to hash receipt: %w", err)
	}
	
	// Verify witness signature
	valid, err := pubKey.Verify([]byte(hash), attestation.WitnessSig)
	if err != nil {
		return fmt.Errorf("failed to verify attestation: %w", err)
	}
	
	if !valid {
		return fmt.Errorf("invalid attestation signature")
	}
	
	return nil
}

// ToJSON serializes the receipt to JSON
func (r *Receipt) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ReceiptFromJSON deserializes a receipt from JSON
func ReceiptFromJSON(data []byte) (*Receipt, error) {
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}
	return &r, nil
}

// Validate checks if the receipt is valid
func (r *Receipt) Validate() error {
	if r.ReceiptID == "" {
		return fmt.Errorf("receipt_id is required")
	}
	if r.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if r.ExecutorID == "" {
		return fmt.Errorf("executor_id is required")
	}
	if r.StartTime.IsZero() {
		return fmt.Errorf("start_time is required")
	}
	if r.EndTime.IsZero() {
		return fmt.Errorf("end_time is required")
	}
	if r.EndTime.Before(r.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	if r.Duration <= 0 {
		return fmt.Errorf("duration must be > 0")
	}
	
	// Verify signature if present
	if len(r.ExecutorSig) > 0 {
		if err := r.Verify(); err != nil {
			return fmt.Errorf("invalid executor signature: %w", err)
		}
	}
	
	// Verify all attestations
	for i := range r.Attestations {
		if err := r.VerifyAttestation(i); err != nil {
			return fmt.Errorf("invalid attestation %d: %w", i, err)
		}
	}
	
	return nil
}

// generateReceiptID creates a unique receipt ID
func generateReceiptID() string {
	data := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 32 character hex string
}

// IsSuccessful returns true if the task executed successfully
func (r *Receipt) IsSuccessful() bool {
	return r.Success && r.ExitCode == 0 && r.Error == ""
}

// GetAttestationCount returns the number of attestations
func (r *Receipt) GetAttestationCount() int {
	return len(r.Attestations)
}

// HasAttestation checks if a specific witness has attested
func (r *Receipt) HasAttestation(witnessID peer.ID) bool {
	for _, att := range r.Attestations {
		if att.WitnessID == witnessID {
			return true
		}
	}
	return false
}
