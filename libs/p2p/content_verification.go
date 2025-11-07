// Package p2p provides content verification for DHT-resolved data
package p2p

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Prometheus metrics
var (
	contentVerifications = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "content_verifications_total",
			Help: "Total content verifications",
		},
		[]string{"result"}, // success, hash_mismatch, signature_invalid
	)

	contentVerificationLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "content_verification_latency_seconds",
			Help:    "Content verification latency",
			Buckets: prometheus.DefBuckets,
		},
	)

	contentSignatureChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "content_signature_checks_total",
			Help: "Total signature checks",
		},
		[]string{"result"}, // valid, invalid, missing
	)

	contentHashChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "content_hash_checks_total",
			Help: "Total hash checks",
		},
		[]string{"result"}, // match, mismatch
	)
)

// VerificationError represents a content verification failure
type VerificationError struct {
	Type    string // hash_mismatch, signature_invalid, etc.
	Message string
	Details map[string]interface{}
}

func (e *VerificationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// SignatureVerifier is an interface for verifying content signatures
type SignatureVerifier interface {
	Verify(ctx context.Context, content []byte) error
}

// ContentVerifier handles content integrity verification
type ContentVerifier struct {
	signatureVerifier SignatureVerifier
	logger            *zap.Logger
}

// ContentVerificationConfig holds verification configuration
type ContentVerificationConfig struct {
	// VerifyHash enables content hash verification
	VerifyHash bool
	// VerifySignature enables signature verification
	VerifySignature bool
	// RequireTimestamp requires valid timestamp
	RequireTimestamp bool
	// MaxClockSkew is maximum allowed clock skew for timestamps
	MaxClockSkew time.Duration
}

// DefaultVerificationConfig returns default configuration
func DefaultVerificationConfig() *ContentVerificationConfig {
	return &ContentVerificationConfig{
		VerifyHash:       true,
		VerifySignature:  true,
		RequireTimestamp: true,
		MaxClockSkew:     5 * time.Minute,
	}
}

// NewContentVerifier creates a new content verifier
func NewContentVerifier(signatureVerifier SignatureVerifier, logger *zap.Logger) *ContentVerifier {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ContentVerifier{
		signatureVerifier: signatureVerifier,
		logger:            logger,
	}
}

// VerifyContent verifies content integrity and authenticity
func (cv *ContentVerifier) VerifyContent(ctx context.Context, expectedCID cid.Cid, content []byte, config *ContentVerificationConfig) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds()
		contentVerificationLatency.Observe(latency)
	}()

	if config == nil {
		config = DefaultVerificationConfig()
	}

	// Verify content hash matches CID
	if config.VerifyHash {
		if err := cv.verifyHash(expectedCID, content); err != nil {
			contentVerifications.WithLabelValues("hash_mismatch").Inc()
			return err
		}
	}

	// Verify signature (for signed content like agent cards)
	if config.VerifySignature {
		if err := cv.verifySignature(ctx, content, config); err != nil {
			contentVerifications.WithLabelValues("signature_invalid").Inc()
			return err
		}
	}

	contentVerifications.WithLabelValues("success").Inc()
	cv.logger.Debug("content verification successful",
		zap.String("cid", expectedCID.String()),
		zap.Int("size_bytes", len(content)),
		zap.Float64("latency_seconds", time.Since(start).Seconds()),
	)

	return nil
}

// verifyHash checks that content hash matches expected CID
func (cv *ContentVerifier) verifyHash(expectedCID cid.Cid, content []byte) error {
	// Compute hash of content
	hash := sha256.Sum256(content)
	
	// Create multihash
	mhash, err := mh.Encode(hash[:], mh.SHA2_256)
	if err != nil {
		contentHashChecks.WithLabelValues("error").Inc()
		return &VerificationError{
			Type:    "hash_computation_failed",
			Message: "failed to compute multihash",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	// Create CID from computed hash
	computedCID := cid.NewCidV1(cid.Raw, mhash)

	// Compare CIDs
	if !computedCID.Equals(expectedCID) {
		contentHashChecks.WithLabelValues("mismatch").Inc()
		cv.logger.Warn("content hash mismatch",
			zap.String("expected_cid", expectedCID.String()),
			zap.String("computed_cid", computedCID.String()),
			zap.Int("content_size", len(content)),
		)
		return &VerificationError{
			Type:    "hash_mismatch",
			Message: "content hash does not match expected CID",
			Details: map[string]interface{}{
				"expected": expectedCID.String(),
				"computed": computedCID.String(),
				"size":     len(content),
			},
		}
	}

	contentHashChecks.WithLabelValues("match").Inc()
	return nil
}

// verifySignature validates cryptographic signature
func (cv *ContentVerifier) verifySignature(ctx context.Context, content []byte, config *ContentVerificationConfig) error {
	if cv.signatureVerifier == nil {
		contentSignatureChecks.WithLabelValues("skipped").Inc()
		return nil
	}

	err := cv.signatureVerifier.Verify(ctx, content)
	if err != nil {
		contentSignatureChecks.WithLabelValues("invalid").Inc()
		cv.logger.Warn("signature verification failed",
			zap.Error(err),
		)
		return &VerificationError{
			Type:    "signature_invalid",
			Message: "signature verification failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	contentSignatureChecks.WithLabelValues("valid").Inc()
	return nil
}

// VerifyAndResolve is a convenience method that resolves and verifies in one call
func (cv *ContentVerifier) VerifyAndResolve(ctx context.Context, node *Node, cidStr string, config *ContentVerificationConfig) ([]byte, error) {
	// Parse CID
	c, err := cid.Decode(cidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CID: %w", err)
	}

	// Resolve content from DHT
	content, err := node.ResolveAgentCard(ctx, cidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve content: %w", err)
	}

	// Verify content
	if err := cv.VerifyContent(ctx, c, content, config); err != nil {
		return nil, err
	}

	return content, nil
}

// VerificationChain represents a chain of content verifications
type VerificationChain struct {
	verifier *ContentVerifier
	logger   *zap.Logger
	chain    []VerificationStep
}

// VerificationStep represents one step in the verification chain
type VerificationStep struct {
	Type      string                 // hash, signature, timestamp, etc.
	Timestamp time.Time
	Result    string                 // success, failure
	Details   map[string]interface{}
}

// NewVerificationChain creates a new verification chain
func NewVerificationChain(verifier *ContentVerifier, logger *zap.Logger) *VerificationChain {
	return &VerificationChain{
		verifier: verifier,
		logger:   logger,
		chain:    make([]VerificationStep, 0),
	}
}

// AddStep adds a verification step to the chain
func (vc *VerificationChain) AddStep(step VerificationStep) {
	vc.chain = append(vc.chain, step)
}

// Verify runs the full verification chain
func (vc *VerificationChain) Verify(ctx context.Context, expectedCID cid.Cid, content []byte, config *ContentVerificationConfig) error {
	startTime := time.Now()

	// Hash verification step
	hashErr := vc.verifier.verifyHash(expectedCID, content)
	vc.AddStep(VerificationStep{
		Type:      "hash",
		Timestamp: time.Now(),
		Result:    resultString(hashErr),
		Details: map[string]interface{}{
			"cid":  expectedCID.String(),
			"size": len(content),
		},
	})
	if hashErr != nil {
		return hashErr
	}

	// Signature verification step
	sigErr := vc.verifier.verifySignature(ctx, content, config)
	vc.AddStep(VerificationStep{
		Type:      "signature",
		Timestamp: time.Now(),
		Result:    resultString(sigErr),
		Details: map[string]interface{}{
			"verifier_present": vc.verifier.signatureVerifier != nil,
		},
	})
	if sigErr != nil {
		return sigErr
	}

	// Final step
	vc.AddStep(VerificationStep{
		Type:      "complete",
		Timestamp: time.Now(),
		Result:    "success",
		Details: map[string]interface{}{
			"total_duration_ms": time.Since(startTime).Milliseconds(),
			"steps":             len(vc.chain),
		},
	})

	vc.logger.Debug("verification chain completed",
		zap.String("cid", expectedCID.String()),
		zap.Int("steps", len(vc.chain)),
		zap.Duration("duration", time.Since(startTime)),
	)

	return nil
}

// GetChain returns the verification chain
func (vc *VerificationChain) GetChain() []VerificationStep {
	return vc.chain
}

// ComputeCID computes the CID for content
func ComputeCID(content []byte) (cid.Cid, error) {
	hash := sha256.Sum256(content)
	mhash, err := mh.Encode(hash[:], mh.SHA2_256)
	if err != nil {
		return cid.Cid{}, err
	}
	return cid.NewCidV1(cid.Raw, mhash), nil
}

// VerifyBatch verifies multiple pieces of content in parallel
func (cv *ContentVerifier) VerifyBatch(ctx context.Context, items []VerificationItem, config *ContentVerificationConfig) []VerificationResult {
	results := make([]VerificationResult, len(items))
	
	// Use buffered channel for parallel verification
	sem := make(chan struct{}, 10) // Limit concurrent verifications
	resultChan := make(chan struct {
		idx int
		res VerificationResult
	}, len(items))

	for i, item := range items {
		go func(idx int, vi VerificationItem) {
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			err := cv.VerifyContent(ctx, vi.CID, vi.Content, config)
			resultChan <- struct {
				idx int
				res VerificationResult
			}{
				idx: idx,
				res: VerificationResult{
					CID:     vi.CID,
					Success: err == nil,
					Error:   err,
				},
			}
		}(i, item)
	}

	// Collect results
	for i := 0; i < len(items); i++ {
		result := <-resultChan
		results[result.idx] = result.res
	}

	return results
}

// VerificationItem represents content to verify
type VerificationItem struct {
	CID     cid.Cid
	Content []byte
}

// VerificationResult represents verification outcome
type VerificationResult struct {
	CID     cid.Cid
	Success bool
	Error   error
}

func resultString(err error) string {
	if err == nil {
		return "success"
	}
	return "failure"
}
