package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewContentVerifier(t *testing.T) {
	logger := zap.NewNop()

	cv := NewContentVerifier(nil, logger)
	assert.NotNil(t, cv)
	assert.NotNil(t, cv.logger)
}

func TestComputeCID(t *testing.T) {
	content := []byte("test content")
	
	c, err := ComputeCID(content)
	require.NoError(t, err)
	assert.True(t, c.Defined())
	assert.Equal(t, uint64(cid.Raw), c.Type())
	
	// Compute again - should be deterministic
	c2, err := ComputeCID(content)
	require.NoError(t, err)
	assert.True(t, c.Equals(c2))
}

func TestVerifyHashMatch(t *testing.T) {
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	content := []byte("test content for hash verification")
	expectedCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	err = cv.verifyHash(expectedCID, content)
	assert.NoError(t, err)
}

func TestVerifyHashMismatch(t *testing.T) {
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	content := []byte("test content")
	expectedCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	// Different content
	differentContent := []byte("different content")
	err = cv.verifyHash(expectedCID, differentContent)
	
	require.Error(t, err)
	verr, ok := err.(*VerificationError)
	require.True(t, ok)
	assert.Equal(t, "hash_mismatch", verr.Type)
}

func TestVerifyContentWithoutSignature(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	content := []byte("test content")
	expectedCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	config := &ContentVerificationConfig{
		VerifyHash:      true,
		VerifySignature: false,
	}
	
	err = cv.VerifyContent(ctx, expectedCID, content, config)
	assert.NoError(t, err)
}

func TestVerifyContentHashOnly(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	content := []byte("test content for hash-only verification")
	expectedCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	config := &ContentVerificationConfig{
		VerifyHash:      true,
		VerifySignature: false,
	}
	
	err = cv.VerifyContent(ctx, expectedCID, content, config)
	assert.NoError(t, err)
}

func TestVerifyContentInvalidHash(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	content := []byte("original content")
	originalCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	// Tampered content
	tamperedContent := []byte("tampered content")
	
	config := DefaultVerificationConfig()
	config.VerifySignature = false // Only test hash
	
	err = cv.VerifyContent(ctx, originalCID, tamperedContent, config)
	require.Error(t, err)
	
	verr, ok := err.(*VerificationError)
	require.True(t, ok)
	assert.Equal(t, "hash_mismatch", verr.Type)
	assert.Contains(t, verr.Details, "expected")
	assert.Contains(t, verr.Details, "computed")
}

func TestVerificationChain(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	
	cv := NewContentVerifier(nil, logger)
	vc := NewVerificationChain(cv, logger)
	
	content := []byte("test content")
	expectedCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	config := &ContentVerificationConfig{
		VerifyHash:      true,
		VerifySignature: false, // Skip signature for this test
	}
	
	err = vc.Verify(ctx, expectedCID, content, config)
	require.NoError(t, err)
	
	chain := vc.GetChain()
	assert.Greater(t, len(chain), 0)
	
	// Check that chain includes hash verification
	foundHash := false
	for _, step := range chain {
		if step.Type == "hash" {
			foundHash = true
			assert.Equal(t, "success", step.Result)
		}
	}
	assert.True(t, foundHash, "chain should include hash verification")
}

func TestVerificationChainFailure(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	vc := NewVerificationChain(cv, logger)
	
	content := []byte("original content")
	expectedCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	// Tampered content
	tamperedContent := []byte("tampered content")
	
	config := &ContentVerificationConfig{
		VerifyHash:      true,
		VerifySignature: false,
	}
	
	err = vc.Verify(ctx, expectedCID, tamperedContent, config)
	require.Error(t, err)
	
	// Chain should still be recorded
	chain := vc.GetChain()
	assert.Greater(t, len(chain), 0)
	
	// Find failed step
	foundFailure := false
	for _, step := range chain {
		if step.Type == "hash" && step.Result == "failure" {
			foundFailure = true
		}
	}
	assert.True(t, foundFailure, "chain should record hash failure")
}

func TestVerifyBatch(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	// Create test items
	items := make([]VerificationItem, 5)
	for i := range items {
		content := []byte(string(rune('a' + i)))
		c, err := ComputeCID(content)
		require.NoError(t, err)
		
		items[i] = VerificationItem{
			CID:     c,
			Content: content,
		}
	}
	
	config := &ContentVerificationConfig{
		VerifyHash:      true,
		VerifySignature: false,
	}
	
	results := cv.VerifyBatch(ctx, items, config)
	
	assert.Len(t, results, 5)
	for i, result := range results {
		assert.True(t, result.Success, "item %d should verify successfully", i)
		assert.NoError(t, result.Error)
	}
}

func TestVerifyBatchWithFailure(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	// Create test items with one mismatch
	items := make([]VerificationItem, 3)
	
	// Item 0: valid
	content0 := []byte("content 0")
	c0, _ := ComputeCID(content0)
	items[0] = VerificationItem{CID: c0, Content: content0}
	
	// Item 1: invalid (content doesn't match CID)
	content1 := []byte("content 1")
	c1, _ := ComputeCID([]byte("different content"))
	items[1] = VerificationItem{CID: c1, Content: content1}
	
	// Item 2: valid
	content2 := []byte("content 2")
	c2, _ := ComputeCID(content2)
	items[2] = VerificationItem{CID: c2, Content: content2}
	
	config := &ContentVerificationConfig{
		VerifyHash:      true,
		VerifySignature: false,
	}
	
	results := cv.VerifyBatch(ctx, items, config)
	
	assert.Len(t, results, 3)
	assert.True(t, results[0].Success)
	assert.False(t, results[1].Success, "item 1 should fail verification")
	assert.True(t, results[2].Success)
}

func TestDefaultVerificationConfig(t *testing.T) {
	config := DefaultVerificationConfig()
	
	assert.True(t, config.VerifyHash)
	assert.True(t, config.VerifySignature)
	assert.True(t, config.RequireTimestamp)
	assert.Equal(t, 5*time.Minute, config.MaxClockSkew)
}

func TestVerificationError(t *testing.T) {
	err := &VerificationError{
		Type:    "hash_mismatch",
		Message: "content hash does not match",
		Details: map[string]interface{}{
			"expected": "abc123",
			"computed": "def456",
		},
	}
	
	errStr := err.Error()
	assert.Contains(t, errStr, "hash_mismatch")
	assert.Contains(t, errStr, "content hash does not match")
}

func TestVerifySignatureWithoutValidator(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger) // No validator
	
	content := []byte("test content")
	config := DefaultVerificationConfig()
	
	err := cv.verifySignature(ctx, content, config)
	assert.NoError(t, err, "should skip signature verification when no validator")
}

func TestVerifyContentDefaultConfig(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	content := []byte("test content")
	expectedCID, err := ComputeCID(content)
	require.NoError(t, err)
	
	// Use default config (nil)
	err = cv.VerifyContent(ctx, expectedCID, content, nil)
	assert.NoError(t, err)
}

func TestComputeCIDDeterministic(t *testing.T) {
	content := []byte("deterministic test content")
	
	// Compute CID multiple times
	cids := make([]cid.Cid, 10)
	for i := range cids {
		c, err := ComputeCID(content)
		require.NoError(t, err)
		cids[i] = c
	}
	
	// All should be equal
	for i := 1; i < len(cids); i++ {
		assert.True(t, cids[0].Equals(cids[i]), "CID computation should be deterministic")
	}
}

func TestVerifyHashWithDifferentContent(t *testing.T) {
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	contents := [][]byte{
		[]byte("content A"),
		[]byte("content B"),
		[]byte(""),
		[]byte("very long content that exceeds typical buffer sizes and includes various characters: !@#$%^&*()"),
	}
	
	for _, content := range contents {
		expectedCID, err := ComputeCID(content)
		require.NoError(t, err)
		
		// Should verify successfully
		err = cv.verifyHash(expectedCID, content)
		assert.NoError(t, err)
		
		// Should fail with different content
		differentContent := append(content, byte('x'))
		err = cv.verifyHash(expectedCID, differentContent)
		assert.Error(t, err)
	}
}

func BenchmarkComputeCID(b *testing.B) {
	content := make([]byte, 1024) // 1KB content
	for i := range content {
		content[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ComputeCID(content)
	}
}

func BenchmarkVerifyHash(b *testing.B) {
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	
	expectedCID, _ := ComputeCID(content)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cv.verifyHash(expectedCID, content)
	}
}

func BenchmarkVerifyBatch(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()
	cv := NewContentVerifier(nil, logger)
	
	// Create 100 test items
	items := make([]VerificationItem, 100)
	for i := range items {
		content := make([]byte, 256)
		for j := range content {
			content[j] = byte((i + j) % 256)
		}
		c, _ := ComputeCID(content)
		items[i] = VerificationItem{CID: c, Content: content}
	}
	
	config := &ContentVerificationConfig{
		VerifyHash:      true,
		VerifySignature: false,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cv.VerifyBatch(ctx, items, config)
	}
}
