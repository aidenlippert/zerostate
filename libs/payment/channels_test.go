package payment

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewChannelManager(t *testing.T) {
	privKey, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	
	peerID, err := peer.IDFromPrivateKey(privKey)
	require.NoError(t, err)
	
	cm := NewChannelManager(peerID, privKey, zap.NewNop())
	assert.NotNil(t, cm)
	assert.Equal(t, peerID, cm.localPeer)
	assert.NotNil(t, cm.channels)
}

func TestOpenChannel(t *testing.T) {
	// Create two peers
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	
	ctx := context.Background()
	depositLocal := 100.0
	depositRemote := 50.0
	expiry := 1 * time.Hour
	
	channel, err := cm1.OpenChannel(ctx, peer2, depositLocal, depositRemote, expiry)
	require.NoError(t, err)
	assert.NotNil(t, channel)
	assert.Equal(t, ChannelStateOpening, channel.State)
	
	// Deposits and balances should match the local and remote deposits
	// regardless of lexicographic ordering
	var localBalance, remoteBalance float64
	if peer1 == channel.PartyA {
		localBalance = channel.BalanceA
		remoteBalance = channel.BalanceB
	} else {
		localBalance = channel.BalanceB
		remoteBalance = channel.BalanceA
	}
	
	assert.Equal(t, depositLocal, localBalance)
	assert.Equal(t, depositRemote, remoteBalance)
	assert.Equal(t, uint64(0), channel.SequenceNum)
}

func TestOpenChannelInvalidDeposits(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	// Deposit below minimum
	_, err = cm1.OpenChannel(ctx, peer2, 0.0001, 100.0, 1*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "below minimum")
	
	// Deposit above maximum
	_, err = cm1.OpenChannel(ctx, peer2, 10000.0, 100.0, 1*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceed maximum")
}

func TestOpenChannelDuplicate(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	// Open first channel
	_, err = cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	// Try to open duplicate
	_, err = cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestActivateChannel(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, ChannelStateOpening, channel.State)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	updatedChannel, err := cm1.GetChannel(channel.ChannelID)
	require.NoError(t, err)
	assert.Equal(t, ChannelStateActive, updatedChannel.State)
}

func TestMakePayment(t *testing.T) {
	// Create two peers
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	// Open and activate channel
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	// Make payment
	payment, err := cm1.MakePayment(ctx, channel.ChannelID, peer2, 30.0, "test payment")
	require.NoError(t, err)
	assert.NotNil(t, payment)
	assert.Equal(t, channel.ChannelID, payment.ChannelID)
	assert.Equal(t, peer1, payment.From)
	assert.Equal(t, peer2, payment.To)
	assert.Equal(t, 30.0, payment.Amount)
	assert.Equal(t, uint64(1), payment.SequenceNum)
	assert.NotEmpty(t, payment.Signature)
	
	// Check updated balances (local peer pays 30, remote peer receives 30)
	updatedChannel, err := cm1.GetChannel(channel.ChannelID)
	require.NoError(t, err)
	
	var localBalance, remoteBalance float64
	if peer1 == updatedChannel.PartyA {
		localBalance = updatedChannel.BalanceA
		remoteBalance = updatedChannel.BalanceB
	} else {
		localBalance = updatedChannel.BalanceB
		remoteBalance = updatedChannel.BalanceA
	}
	
	assert.Equal(t, 70.0, localBalance)  // 100 - 30
	assert.Equal(t, 80.0, remoteBalance) // 50 + 30
	assert.Equal(t, uint64(1), updatedChannel.SequenceNum)
}

func TestMakePaymentInsufficientBalance(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	channel, err := cm1.OpenChannel(ctx, peer2, 50.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	// Try to pay more than balance
	_, err = cm1.MakePayment(ctx, channel.ChannelID, peer2, 100.0, "overpayment")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")
}

func TestMakePaymentChannelNotActive(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	// Don't activate, try to pay
	_, err = cm1.MakePayment(ctx, channel.ChannelID, peer2, 30.0, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestMakeMultiplePayments(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	// Make multiple payments
	payment1, err := cm1.MakePayment(ctx, channel.ChannelID, peer2, 10.0, "payment 1")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), payment1.SequenceNum)
	
	payment2, err := cm1.MakePayment(ctx, channel.ChannelID, peer2, 20.0, "payment 2")
	require.NoError(t, err)
	assert.Equal(t, uint64(2), payment2.SequenceNum)
	
	payment3, err := cm1.MakePayment(ctx, channel.ChannelID, peer2, 15.0, "payment 3")
	require.NoError(t, err)
	assert.Equal(t, uint64(3), payment3.SequenceNum)
	
	// Check final balances
	updatedChannel, err := cm1.GetChannel(channel.ChannelID)
	require.NoError(t, err)
	
	var localBalance, remoteBalance float64
	if peer1 == updatedChannel.PartyA {
		localBalance = updatedChannel.BalanceA
		remoteBalance = updatedChannel.BalanceB
	} else {
		localBalance = updatedChannel.BalanceB
		remoteBalance = updatedChannel.BalanceA
	}
	
	assert.Equal(t, 55.0, localBalance)  // 100 - 10 - 20 - 15
	assert.Equal(t, 95.0, remoteBalance) // 50 + 10 + 20 + 15
}

func TestPaymentHashAndVerify(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	payment, err := cm1.MakePayment(ctx, channel.ChannelID, peer2, 30.0, "test")
	require.NoError(t, err)
	
	// Test hash
	hash, err := payment.Hash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	
	// Test verify with correct key
	pubKey := privKey1.GetPublic()
	err = payment.Verify(pubKey)
	assert.NoError(t, err)
	
	// Test verify with wrong key
	wrongPrivKey, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	wrongPubKey := wrongPrivKey.GetPublic()
	err = payment.Verify(wrongPubKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestCloseChannel(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	// Make a payment
	_, err = cm1.MakePayment(ctx, channel.ChannelID, peer2, 30.0, "test")
	require.NoError(t, err)
	
	// Close channel
	err = cm1.CloseChannel(ctx, channel.ChannelID, "normal_closure")
	require.NoError(t, err)
	
	updatedChannel, err := cm1.GetChannel(channel.ChannelID)
	require.NoError(t, err)
	assert.Equal(t, ChannelStateClosed, updatedChannel.State)
	
	// Verify can't make payment on closed channel
	_, err = cm1.MakePayment(ctx, channel.ChannelID, peer2, 10.0, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestListChannels(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	privKey3, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer3, err := peer.IDFromPrivateKey(privKey3)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	// Open multiple channels
	_, err = cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	_, err = cm1.OpenChannel(ctx, peer3, 200.0, 100.0, 2*time.Hour)
	require.NoError(t, err)
	
	channels := cm1.ListChannels()
	assert.Len(t, channels, 2)
}

func TestChannelStats(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 1*time.Hour)
	require.NoError(t, err)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	stats := cm1.Stats()
	assert.Equal(t, 1, stats["total_channels"])
	assert.Equal(t, 1, stats["active_channels"])
	assert.Equal(t, 150.0, stats["total_balance"])
}

func TestChannelExpiry(t *testing.T) {
	privKey1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer1, err := peer.IDFromPrivateKey(privKey1)
	require.NoError(t, err)
	
	privKey2, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peer2, err := peer.IDFromPrivateKey(privKey2)
	require.NoError(t, err)
	
	cm1 := NewChannelManager(peer1, privKey1, zap.NewNop())
	ctx := context.Background()
	
	// Open channel with very short expiry
	channel, err := cm1.OpenChannel(ctx, peer2, 100.0, 50.0, 10*time.Millisecond)
	require.NoError(t, err)
	
	err = cm1.ActivateChannel(ctx, channel.ChannelID)
	require.NoError(t, err)
	
	// Wait for expiry
	time.Sleep(20 * time.Millisecond)
	
	// Try to make payment
	_, err = cm1.MakePayment(ctx, channel.ChannelID, peer2, 10.0, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestDefaultChannelConfig(t *testing.T) {
	config := DefaultChannelConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 24*time.Hour, config.DefaultExpiry)
	assert.Equal(t, 0.001, config.MinDeposit)
	assert.Equal(t, 1000.0, config.MaxDeposit)
}
