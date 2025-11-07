package payment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Prometheus metrics
var (
	channelsOpenedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payment_channels_opened_total",
		Help: "Total number of payment channels opened",
	})
	
	channelsClosedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_channels_closed_total",
		Help: "Total number of payment channels closed",
	}, []string{"reason"})
	
	channelBalanceGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_channel_balance",
		Help: "Current balance in payment channel",
	}, []string{"channel_id", "party"})
	
	paymentsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_payments_processed_total",
		Help: "Total number of payments processed",
	}, []string{"status"})
	
	paymentAmountTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payment_amount_total",
		Help: "Total payment amount processed",
	})
)

// ChannelState represents the state of a payment channel
type ChannelState string

const (
	ChannelStateOpening  ChannelState = "opening"
	ChannelStateActive   ChannelState = "active"
	ChannelStateClosing  ChannelState = "closing"
	ChannelStateClosed   ChannelState = "closed"
	ChannelStateDisputed ChannelState = "disputed"
)

// PaymentChannel represents a bidirectional payment channel between two parties
type PaymentChannel struct {
	ChannelID   string       `json:"channel_id"`
	PartyA      peer.ID      `json:"party_a"` // Payer (task creator)
	PartyB      peer.ID      `json:"party_b"` // Payee (task executor)
	State       ChannelState `json:"state"`
	
	// Initial deposits
	DepositA    float64      `json:"deposit_a"` // Party A's deposit
	DepositB    float64      `json:"deposit_b"` // Party B's deposit
	
	// Current balances (off-chain)
	BalanceA    float64      `json:"balance_a"` // Party A's current balance
	BalanceB    float64      `json:"balance_b"` // Party B's current balance
	
	// Metadata
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	ExpiresAt   time.Time    `json:"expires_at"`
	
	// State tracking
	SequenceNum uint64       `json:"sequence_num"` // Monotonic sequence for updates
	
	// Signatures for current state
	SignatureA  []byte       `json:"signature_a,omitempty"`
	SignatureB  []byte       `json:"signature_b,omitempty"`
	
	mu          sync.RWMutex `json:"-"`
}

// Payment represents a single payment within a channel
type Payment struct {
	PaymentID   string    `json:"payment_id"`
	ChannelID   string    `json:"channel_id"`
	From        peer.ID   `json:"from"`
	To          peer.ID   `json:"to"`
	Amount      float64   `json:"amount"`
	SequenceNum uint64    `json:"sequence_num"`
	Timestamp   time.Time `json:"timestamp"`
	Memo        string    `json:"memo,omitempty"` // Optional: task ID, receipt ID, etc.
	Signature   []byte    `json:"signature"`
}

// ChannelManager manages multiple payment channels
type ChannelManager struct {
	localPeer peer.ID
	privKey   crypto.PrivKey
	channels  map[string]*PaymentChannel
	logger    *zap.Logger
	mu        sync.RWMutex
}

// ChannelConfig holds configuration for payment channels
type ChannelConfig struct {
	DefaultExpiry time.Duration // Default channel expiry (e.g., 24 hours)
	MinDeposit    float64       // Minimum deposit required
	MaxDeposit    float64       // Maximum deposit allowed
}

// DefaultChannelConfig returns default channel configuration
func DefaultChannelConfig() *ChannelConfig {
	return &ChannelConfig{
		DefaultExpiry: 24 * time.Hour,
		MinDeposit:    0.001, // 0.001 currency units
		MaxDeposit:    1000.0, // 1000 currency units
	}
}

// NewChannelManager creates a new payment channel manager
func NewChannelManager(localPeer peer.ID, privKey crypto.PrivKey, logger *zap.Logger) *ChannelManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &ChannelManager{
		localPeer: localPeer,
		privKey:   privKey,
		channels:  make(map[string]*PaymentChannel),
		logger:    logger,
	}
}

// OpenChannel opens a new payment channel with another peer
func (cm *ChannelManager) OpenChannel(ctx context.Context, otherPeer peer.ID, depositA, depositB float64, expiry time.Duration) (*PaymentChannel, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Validate deposits
	config := DefaultChannelConfig()
	if depositA < config.MinDeposit || depositB < config.MinDeposit {
		return nil, fmt.Errorf("deposits below minimum: %f (min: %f)", min(depositA, depositB), config.MinDeposit)
	}
	if depositA > config.MaxDeposit || depositB > config.MaxDeposit {
		return nil, fmt.Errorf("deposits exceed maximum: %f (max: %f)", max(depositA, depositB), config.MaxDeposit)
	}
	
	// Determine party ordering (consistent for both peers)
	var partyA, partyB peer.ID
	var depositPartyA, depositPartyB float64
	
	if cm.localPeer < otherPeer {
		partyA = cm.localPeer
		partyB = otherPeer
		depositPartyA = depositA
		depositPartyB = depositB
	} else {
		partyA = otherPeer
		partyB = cm.localPeer
		depositPartyA = depositB
		depositPartyB = depositA
	}
	
	// Generate channel ID
	channelID := generateChannelID(partyA, partyB)
	
	// Check if channel already exists
	if _, exists := cm.channels[channelID]; exists {
		return nil, fmt.Errorf("channel already exists with peer %s", otherPeer)
	}
	
	now := time.Now()
	channel := &PaymentChannel{
		ChannelID:   channelID,
		PartyA:      partyA,
		PartyB:      partyB,
		State:       ChannelStateOpening,
		DepositA:    depositPartyA,
		DepositB:    depositPartyB,
		BalanceA:    depositPartyA,
		BalanceB:    depositPartyB,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(expiry),
		SequenceNum: 0,
	}
	
	cm.channels[channelID] = channel
	
	channelsOpenedTotal.Inc()
	channelBalanceGauge.WithLabelValues(channelID, "party_a").Set(depositA)
	channelBalanceGauge.WithLabelValues(channelID, "party_b").Set(depositB)
	
	cm.logger.Info("Payment channel opened",
		zap.String("channel_id", channelID),
		zap.String("party_a", partyA.String()),
		zap.String("party_b", partyB.String()),
		zap.Float64("deposit_a", depositA),
		zap.Float64("deposit_b", depositB),
	)
	
	return channel, nil
}

// GetChannel retrieves a payment channel by ID
func (cm *ChannelManager) GetChannel(channelID string) (*PaymentChannel, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	channel, exists := cm.channels[channelID]
	if !exists {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}
	
	return channel, nil
}

// MakePayment creates a payment within a channel
func (cm *ChannelManager) MakePayment(ctx context.Context, channelID string, to peer.ID, amount float64, memo string) (*Payment, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	channel, exists := cm.channels[channelID]
	if !exists {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}
	
	channel.mu.Lock()
	defer channel.mu.Unlock()
	
	// Verify channel is active
	if channel.State != ChannelStateActive {
		return nil, fmt.Errorf("channel not active: %s", channel.State)
	}
	
	// Verify not expired
	if time.Now().After(channel.ExpiresAt) {
		return nil, fmt.Errorf("channel expired")
	}
	
	// Determine sender and update balances
	var newBalanceA, newBalanceB float64
	if cm.localPeer == channel.PartyA {
		// Party A sending to Party B
		if to != channel.PartyB {
			return nil, fmt.Errorf("invalid recipient: expected %s", channel.PartyB)
		}
		if channel.BalanceA < amount {
			return nil, fmt.Errorf("insufficient balance: %f < %f", channel.BalanceA, amount)
		}
		newBalanceA = channel.BalanceA - amount
		newBalanceB = channel.BalanceB + amount
	} else {
		// Party B sending to Party A
		if to != channel.PartyA {
			return nil, fmt.Errorf("invalid recipient: expected %s", channel.PartyA)
		}
		if channel.BalanceB < amount {
			return nil, fmt.Errorf("insufficient balance: %f < %f", channel.BalanceB, amount)
		}
		newBalanceA = channel.BalanceA + amount
		newBalanceB = channel.BalanceB - amount
	}
	
	// Create payment
	payment := &Payment{
		PaymentID:   generatePaymentID(),
		ChannelID:   channelID,
		From:        cm.localPeer,
		To:          to,
		Amount:      amount,
		SequenceNum: channel.SequenceNum + 1,
		Timestamp:   time.Now(),
		Memo:        memo,
	}
	
	// Sign payment
	hash, err := payment.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to hash payment: %w", err)
	}
	
	sig, err := cm.privKey.Sign([]byte(hash))
	if err != nil {
		return nil, fmt.Errorf("failed to sign payment: %w", err)
	}
	payment.Signature = sig
	
	// Update channel state
	channel.BalanceA = newBalanceA
	channel.BalanceB = newBalanceB
	channel.SequenceNum = payment.SequenceNum
	channel.UpdatedAt = time.Now()
	
	// Update metrics
	channelBalanceGauge.WithLabelValues(channelID, "party_a").Set(newBalanceA)
	channelBalanceGauge.WithLabelValues(channelID, "party_b").Set(newBalanceB)
	paymentsProcessedTotal.WithLabelValues("success").Inc()
	paymentAmountTotal.Add(amount)
	
	cm.logger.Info("Payment made",
		zap.String("payment_id", payment.PaymentID),
		zap.String("channel_id", channelID),
		zap.String("from", cm.localPeer.String()),
		zap.String("to", to.String()),
		zap.Float64("amount", amount),
		zap.Uint64("sequence", payment.SequenceNum),
	)
	
	return payment, nil
}

// ActivateChannel transitions a channel from opening to active state
func (cm *ChannelManager) ActivateChannel(ctx context.Context, channelID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	channel, exists := cm.channels[channelID]
	if !exists {
		return fmt.Errorf("channel not found: %s", channelID)
	}
	
	channel.mu.Lock()
	defer channel.mu.Unlock()
	
	if channel.State != ChannelStateOpening {
		return fmt.Errorf("channel not in opening state: %s", channel.State)
	}
	
	channel.State = ChannelStateActive
	channel.UpdatedAt = time.Now()
	
	cm.logger.Info("Channel activated",
		zap.String("channel_id", channelID),
	)
	
	return nil
}

// CloseChannel closes a payment channel and settles balances
func (cm *ChannelManager) CloseChannel(ctx context.Context, channelID string, reason string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	channel, exists := cm.channels[channelID]
	if !exists {
		return fmt.Errorf("channel not found: %s", channelID)
	}
	
	channel.mu.Lock()
	defer channel.mu.Unlock()
	
	if channel.State == ChannelStateClosed {
		return fmt.Errorf("channel already closed")
	}
	
	channel.State = ChannelStateClosed
	channel.UpdatedAt = time.Now()
	
	channelsClosedTotal.WithLabelValues(reason).Inc()
	
	cm.logger.Info("Channel closed",
		zap.String("channel_id", channelID),
		zap.String("reason", reason),
		zap.Float64("final_balance_a", channel.BalanceA),
		zap.Float64("final_balance_b", channel.BalanceB),
	)
	
	return nil
}

// ListChannels returns all payment channels
func (cm *ChannelManager) ListChannels() []*PaymentChannel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	channels := make([]*PaymentChannel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	
	return channels
}

// Stats returns channel manager statistics
func (cm *ChannelManager) Stats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_channels": len(cm.channels),
		"active_channels": 0,
		"total_balance": 0.0,
	}
	
	for _, ch := range cm.channels {
		if ch.State == ChannelStateActive {
			stats["active_channels"] = stats["active_channels"].(int) + 1
		}
		stats["total_balance"] = stats["total_balance"].(float64) + ch.BalanceA + ch.BalanceB
	}
	
	return stats
}

// Hash returns the SHA256 hash of the payment
func (p *Payment) Hash() (string, error) {
	data, err := json.Marshal(struct {
		PaymentID   string
		ChannelID   string
		From        string
		To          string
		Amount      float64
		SequenceNum uint64
		Timestamp   time.Time
		Memo        string
	}{
		PaymentID:   p.PaymentID,
		ChannelID:   p.ChannelID,
		From:        p.From.String(),
		To:          p.To.String(),
		Amount:      p.Amount,
		SequenceNum: p.SequenceNum,
		Timestamp:   p.Timestamp,
		Memo:        p.Memo,
	})
	if err != nil {
		return "", err
	}
	
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// Verify verifies the payment signature
func (p *Payment) Verify(pubKey crypto.PubKey) error {
	hash, err := p.Hash()
	if err != nil {
		return fmt.Errorf("failed to hash payment: %w", err)
	}
	
	valid, err := pubKey.Verify([]byte(hash), p.Signature)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}
	
	if !valid {
		return fmt.Errorf("invalid signature")
	}
	
	return nil
}

// Helper functions
func generateChannelID(partyA, partyB peer.ID) string {
	// Use deterministic channel ID based only on parties (not time)
	// This ensures the same two parties always get the same channel ID
	data := fmt.Sprintf("%s-%s", partyA.String(), partyB.String())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

func generatePaymentID() string {
	data := fmt.Sprintf("payment-%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
