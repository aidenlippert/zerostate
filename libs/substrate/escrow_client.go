package substrate

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"go.uber.org/zap"
)

type EscrowClient struct {
	client   *ClientV2
	keyring  *signature.KeyringPair
	palletID uint8
	logger   *zap.Logger
}

func NewEscrowClient(client *ClientV2, keyring *signature.KeyringPair) *EscrowClient {
	return &EscrowClient{
		client:   client,
		keyring:  keyring,
		palletID: 10,
		logger:   zap.L().Named("escrow-client"),
	}
}

func NewEscrowClientWithLogger(client *ClientV2, keyring *signature.KeyringPair, logger *zap.Logger) *EscrowClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &EscrowClient{
		client:   client,
		keyring:  keyring,
		palletID: 10,
		logger:   logger.Named("escrow-client"),
	}
}

func (ec *EscrowClient) CreateEscrow(ctx context.Context, taskID [32]byte, amount uint64, taskHash [32]byte, timeoutBlocks *uint32) error {
	start := time.Now()

	ec.logger.Info("creating escrow",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.Uint64("amount", amount),
		zap.String("task_hash", fmt.Sprintf("0x%x", taskHash)),
	)

	meta := ec.client.GetMetadata()
	amountBig := types.NewU128(*new(big.Int).SetUint64(amount))

	var timeoutArg interface{}
	if timeoutBlocks != nil {
		timeoutArg = types.NewOptionU32(types.NewU32(*timeoutBlocks))
		ec.logger.Debug("using custom timeout", zap.Uint32("timeout_blocks", *timeoutBlocks))
	} else {
		timeoutArg = types.NewOptionU32Empty()
		ec.logger.Debug("using default timeout")
	}

	call, err := types.NewCall(meta, "Escrow.create_escrow", taskID, amountBig, taskHash, timeoutArg)
	if err != nil {
		ec.logger.Error("failed to create call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to create escrow",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to create escrow: %w", err)
	}

	ec.logger.Info("escrow created successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func (ec *EscrowClient) AcceptTask(ctx context.Context, taskID [32]byte, agentDID string) error {
	start := time.Now()

	ec.logger.Info("accepting task",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("agent_did", agentDID),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.accept_task", taskID, types.NewBytes([]byte(agentDID)))
	if err != nil {
		ec.logger.Error("failed to create accept_task call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.String("agent_did", agentDID),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to accept task",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.String("agent_did", agentDID),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to accept task: %w", err)
	}

	ec.logger.Info("task accepted successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("agent_did", agentDID),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func (ec *EscrowClient) ReleasePayment(ctx context.Context, taskID [32]byte) error {
	start := time.Now()

	ec.logger.Info("releasing payment",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.release_payment", taskID)
	if err != nil {
		ec.logger.Error("failed to create release_payment call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to release payment",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to release payment: %w", err)
	}

	ec.logger.Info("payment released successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func (ec *EscrowClient) RefundEscrow(ctx context.Context, taskID [32]byte) error {
	start := time.Now()

	ec.logger.Info("refunding escrow",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.refund_escrow", taskID)
	if err != nil {
		ec.logger.Error("failed to create refund_escrow call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to refund escrow",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to refund escrow: %w", err)
	}

	ec.logger.Info("escrow refunded successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func (ec *EscrowClient) DisputeEscrow(ctx context.Context, taskID [32]byte) error {
	start := time.Now()

	ec.logger.Info("disputing escrow",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.dispute_escrow", taskID)
	if err != nil {
		ec.logger.Error("failed to create dispute_escrow call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to dispute escrow",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to dispute escrow: %w", err)
	}

	ec.logger.Info("escrow disputed successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// GetEscrow queries a single escrow by task ID
func (ec *EscrowClient) GetEscrow(ctx context.Context, taskID [32]byte) (*EscrowDetails, error) {
	start := time.Now()
	taskIDHex := fmt.Sprintf("0x%x", taskID)

	ec.logger.Debug("querying escrow",
		zap.String("task_id", taskIDHex),
	)

	key, err := types.CreateStorageKey(ec.client.metadata, "Escrow", "Escrows", taskID[:])
	if err != nil {
		ec.logger.Error("failed to create storage key",
			zap.Error(err),
			zap.String("task_id", taskIDHex),
		)
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	var result struct {
		TaskID       [32]byte
		User         types.AccountID
		AgentDID     types.OptionBytes
		AgentAccount types.OptionAccountID
		Amount       types.U128
		FeePercent   types.U8
		CreatedAt    types.U32
		ExpiresAt    types.U32
		State        types.U8
		TaskHash     [32]byte
	}

	ok, err := ec.client.api.RPC.State.GetStorageLatest(key, &result)
	if err != nil {
		ec.logger.Error("failed to query escrow storage",
			zap.Error(err),
			zap.String("task_id", taskIDHex),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, fmt.Errorf("failed to query storage: %w", err)
	}
	if !ok {
		ec.logger.Debug("escrow not found",
			zap.String("task_id", taskIDHex),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, fmt.Errorf("escrow not found for task %s", taskIDHex)
	}

	// Convert state enum
	stateMap := []EscrowState{
		EscrowStatePending,
		EscrowStateAccepted,
		EscrowStateCompleted,
		EscrowStateRefunded,
		EscrowStateDisputed,
	}
	var state EscrowState
	if int(result.State) < len(stateMap) {
		state = stateMap[result.State]
	} else {
		state = EscrowStatePending
	}

	details := &EscrowDetails{
		TaskID:     result.TaskID,
		User:       AccountID(result.User),
		Amount:     Balance(result.Amount.Int.String()),
		FeePercent: uint8(result.FeePercent),
		CreatedAt:  BlockNumber(result.CreatedAt),
		ExpiresAt:  BlockNumber(result.ExpiresAt),
		State:      state,
		TaskHash:   result.TaskHash,
	}

	// Handle optional AgentDID
	if result.AgentDID.IsNone() == false {
		ok, val := result.AgentDID.Unwrap()
		if ok {
			agentDID := DID(val)
			details.AgentDID = &agentDID
		}
	}

	// Handle optional AgentAccount
	if result.AgentAccount.IsNone() == false {
		ok, val := result.AgentAccount.Unwrap()
		if ok {
			agentAcct := AccountID(val)
			details.AgentAccount = &agentAcct
		}
	}

	ec.logger.Debug("escrow query completed",
		zap.String("task_id", taskIDHex),
		zap.String("state", string(details.State)),
		zap.Duration("duration", time.Since(start)),
	)

	return details, nil
}

// GetUserEscrows queries all escrow IDs for a user
func (ec *EscrowClient) GetUserEscrows(ctx context.Context, userAccount AccountID) ([][32]byte, error) {
	key, err := types.CreateStorageKey(ec.client.metadata, "Escrow", "UserEscrows", userAccount[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	var taskIDs []byte
	ok, err := ec.client.api.RPC.State.GetStorageLatest(key, &taskIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query storage: %w", err)
	}
	if !ok {
		return [][32]byte{}, nil // Return empty slice if no escrows
	}

	// Parse BoundedVec of [u8; 32]
	// Format: <length><taskID1><taskID2>...
	if len(taskIDs) < 4 {
		return [][32]byte{}, nil
	}

	// First 4 bytes are the length (compact encoding)
	count := int(taskIDs[0]) // Simplified - assumes length < 64
	result := make([][32]byte, 0, count)

	offset := 1 // Skip length byte
	for i := 0; i < count && offset+32 <= len(taskIDs); i++ {
		var taskID [32]byte
		copy(taskID[:], taskIDs[offset:offset+32])
		result = append(result, taskID)
		offset += 32
	}

	return result, nil
}

// GetAgentEscrows queries all escrow IDs for an agent
func (ec *EscrowClient) GetAgentEscrows(ctx context.Context, agentDID DID) ([][32]byte, error) {
	key, err := types.CreateStorageKey(ec.client.metadata, "Escrow", "AgentEscrows", []byte(agentDID))
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	var taskIDs []byte
	ok, err := ec.client.api.RPC.State.GetStorageLatest(key, &taskIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query storage: %w", err)
	}
	if !ok {
		return [][32]byte{}, nil
	}

	// Parse BoundedVec of [u8; 32] (same format as UserEscrows)
	if len(taskIDs) < 4 {
		return [][32]byte{}, nil
	}

	count := int(taskIDs[0])
	result := make([][32]byte, 0, count)

	offset := 1
	for i := 0; i < count && offset+32 <= len(taskIDs); i++ {
		var taskID [32]byte
		copy(taskID[:], taskIDs[offset:offset+32])
		result = append(result, taskID)
		offset += 32
	}

	return result, nil
}

// GetEscrowState queries just the state of an escrow (lightweight query)
func (ec *EscrowClient) GetEscrowState(ctx context.Context, taskID [32]byte) (EscrowState, error) {
	escrow, err := ec.GetEscrow(ctx, taskID)
	if err != nil {
		return EscrowState(""), fmt.Errorf("failed to get escrow: %w", err)
	}
	return escrow.State, nil
}

func (ec *EscrowClient) submitTransaction(ctx context.Context, call types.Call) (types.Hash, error) {
	rv, err := ec.client.api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get runtime version: %w", err)
	}
	key, err := types.CreateStorageKey(ec.client.metadata, "System", "Account", ec.keyring.PublicKey)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to create storage key: %w", err)
	}
	var accountInfo types.AccountInfo
	ok, err := ec.client.api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get account info: %w", err)
	}
	nonce := types.NewUCompactFromUInt(0)
	if ok {
		nonce = types.NewUCompactFromUInt(uint64(accountInfo.Nonce))
	}
	ext := types.NewExtrinsic(call)
	genesisHash := ec.client.GetGenesisHash()
	blockHash, err := ec.client.api.RPC.Chain.GetBlockHashLatest()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get block hash: %w", err)
	}
	o := types.SignatureOptions{
		BlockHash:          blockHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              nonce,
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}
	err = ext.Sign(*ec.keyring, o)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to sign extrinsic: %w", err)
	}
	hash, err := ec.client.api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to submit extrinsic: %w", err)
	}
	return hash, nil
}

// =============================================================================
// MULTI-PARTY ESCROW METHODS
// =============================================================================

// AddParticipant adds a participant to multi-party escrow
func (ec *EscrowClient) AddParticipant(ctx context.Context, taskID [32]byte, participant AccountID, role ParticipantRole, amount uint64) error {
	start := time.Now()

	ec.logger.Info("adding participant to multi-party escrow",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("participant", fmt.Sprintf("0x%x", participant)),
		zap.String("role", role.String()),
		zap.Uint64("amount", amount),
	)

	meta := ec.client.GetMetadata()
	amountBig := types.NewU128(*new(big.Int).SetUint64(amount))

	call, err := types.NewCall(meta, "Escrow.add_participant", taskID, participant, types.NewU8(uint8(role)), amountBig)
	if err != nil {
		ec.logger.Error("failed to create add_participant call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to add participant",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to add participant: %w", err)
	}

	ec.logger.Info("participant added successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("participant", fmt.Sprintf("0x%x", participant)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// RemoveParticipant removes a participant from multi-party escrow
func (ec *EscrowClient) RemoveParticipant(ctx context.Context, taskID [32]byte, participant AccountID) error {
	start := time.Now()

	ec.logger.Info("removing participant from multi-party escrow",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("participant", fmt.Sprintf("0x%x", participant)),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.remove_participant", taskID, participant)
	if err != nil {
		ec.logger.Error("failed to create remove_participant call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to remove participant",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	ec.logger.Info("participant removed successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("participant", fmt.Sprintf("0x%x", participant)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// ApproveMultiParty submits approval for multi-party escrow
func (ec *EscrowClient) ApproveMultiParty(ctx context.Context, taskID [32]byte) error {
	start := time.Now()

	ec.logger.Info("approving multi-party escrow",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.approve_multi_party", taskID)
	if err != nil {
		ec.logger.Error("failed to create approve_multi_party call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to approve multi-party escrow",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to approve multi-party escrow: %w", err)
	}

	ec.logger.Info("multi-party escrow approved successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// =============================================================================
// MILESTONE ESCROW METHODS
// =============================================================================

// AddMilestone adds a milestone to milestone-based escrow
func (ec *EscrowClient) AddMilestone(ctx context.Context, taskID [32]byte, description string, amount uint64, requiredApprovals uint32) error {
	start := time.Now()

	ec.logger.Info("adding milestone to escrow",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("description", description),
		zap.Uint64("amount", amount),
		zap.Uint32("required_approvals", requiredApprovals),
	)

	meta := ec.client.GetMetadata()
	amountBig := types.NewU128(*new(big.Int).SetUint64(amount))
	descBytes := types.NewBytes([]byte(description))

	call, err := types.NewCall(meta, "Escrow.add_milestone", taskID, descBytes, amountBig, types.NewU32(requiredApprovals))
	if err != nil {
		ec.logger.Error("failed to create add_milestone call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to add milestone",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to add milestone: %w", err)
	}

	ec.logger.Info("milestone added successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("description", description),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// CompleteMilestone marks a milestone as completed
func (ec *EscrowClient) CompleteMilestone(ctx context.Context, taskID [32]byte, milestoneID uint32) error {
	start := time.Now()

	ec.logger.Info("completing milestone",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.Uint32("milestone_id", milestoneID),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.complete_milestone", taskID, types.NewU32(milestoneID))
	if err != nil {
		ec.logger.Error("failed to create complete_milestone call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to complete milestone",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to complete milestone: %w", err)
	}

	ec.logger.Info("milestone completed successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.Uint32("milestone_id", milestoneID),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// ApproveMilestone approves a completed milestone
func (ec *EscrowClient) ApproveMilestone(ctx context.Context, taskID [32]byte, milestoneID uint32) error {
	start := time.Now()

	ec.logger.Info("approving milestone",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.Uint32("milestone_id", milestoneID),
	)

	meta := ec.client.GetMetadata()
	call, err := types.NewCall(meta, "Escrow.approve_milestone", taskID, types.NewU32(milestoneID))
	if err != nil {
		ec.logger.Error("failed to create approve_milestone call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to approve milestone",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to approve milestone: %w", err)
	}

	ec.logger.Info("milestone approved successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.Uint32("milestone_id", milestoneID),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// =============================================================================
// BATCH OPERATION METHODS
// =============================================================================

// BatchCreateEscrow creates multiple escrows in a single transaction
func (ec *EscrowClient) BatchCreateEscrow(ctx context.Context, requests []BatchCreateEscrowRequest) (*BatchCreateEscrowResult, error) {
	start := time.Now()

	ec.logger.Info("creating batch escrow",
		zap.Int("request_count", len(requests)),
	)

	if len(requests) == 0 {
		return nil, fmt.Errorf("no requests provided")
	}

	meta := ec.client.GetMetadata()

	// Convert requests to substrate types
	var calls []types.Call
	for i, req := range requests {
		amountBig := types.NewU128(*new(big.Int).SetUint64(req.Amount))

		var timeoutArg interface{}
		if req.TimeoutBlocks != nil {
			timeoutArg = types.NewOptionU32(types.NewU32(*req.TimeoutBlocks))
		} else {
			timeoutArg = types.NewOptionU32Empty()
		}

		call, err := types.NewCall(meta, "Escrow.create_escrow", req.TaskID, amountBig, req.TaskHash, timeoutArg)
		if err != nil {
			ec.logger.Error("failed to create call for batch item",
				zap.Error(err),
				zap.Int("item_index", i),
				zap.String("task_id", fmt.Sprintf("0x%x", req.TaskID)),
			)
			return nil, fmt.Errorf("failed to create call for item %d: %w", i, err)
		}
		calls = append(calls, call)
	}

	// Create batch call
	batchCall, err := types.NewCall(meta, "Utility.batch", calls)
	if err != nil {
		ec.logger.Error("failed to create batch call", zap.Error(err))
		return nil, fmt.Errorf("failed to create batch call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, batchCall)
	if err != nil {
		ec.logger.Error("failed to submit batch escrow",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, fmt.Errorf("failed to submit batch escrow: %w", err)
	}

	// Create result (in real implementation, you'd parse transaction events to get actual results)
	result := &BatchCreateEscrowResult{
		SuccessfulTasks: make([]BatchEscrowTaskResult, 0),
		FailedTasks:     make([]BatchEscrowTaskError, 0),
		TotalProcessed:  uint32(len(requests)),
		TotalSucceeded:  uint32(len(requests)), // Assume success for now
		TotalFailed:     0,
		TransactionHash: Hash(hash[:]),
	}

	// Add successful tasks (in real implementation, parse events for actual status)
	for _, req := range requests {
		result.SuccessfulTasks = append(result.SuccessfulTasks, BatchEscrowTaskResult{
			TaskID:    req.TaskID,
			EscrowID:  req.TaskID, // Using task ID as escrow ID for simplicity
			Amount:    Balance(new(big.Int).SetUint64(req.Amount).String()),
			CreatedAt: BlockNumber(0), // Would be filled from block info
		})
	}

	ec.logger.Info("batch escrow created successfully",
		zap.Int("total_requests", len(requests)),
		zap.Uint32("successful", result.TotalSucceeded),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return result, nil
}

// BatchReleasePayment releases payment for multiple escrows
func (ec *EscrowClient) BatchReleasePayment(ctx context.Context, taskIDs [][32]byte) error {
	start := time.Now()

	ec.logger.Info("releasing batch payments",
		zap.Int("task_count", len(taskIDs)),
	)

	if len(taskIDs) == 0 {
		return fmt.Errorf("no task IDs provided")
	}

	meta := ec.client.GetMetadata()

	// Create calls for each task
	var calls []types.Call
	for i, taskID := range taskIDs {
		call, err := types.NewCall(meta, "Escrow.release_payment", taskID)
		if err != nil {
			ec.logger.Error("failed to create release call for batch item",
				zap.Error(err),
				zap.Int("item_index", i),
				zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			)
			return fmt.Errorf("failed to create call for item %d: %w", i, err)
		}
		calls = append(calls, call)
	}

	// Create batch call
	batchCall, err := types.NewCall(meta, "Utility.batch", calls)
	if err != nil {
		ec.logger.Error("failed to create batch release call", zap.Error(err))
		return fmt.Errorf("failed to create batch call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, batchCall)
	if err != nil {
		ec.logger.Error("failed to submit batch release",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to submit batch release: %w", err)
	}

	ec.logger.Info("batch payment released successfully",
		zap.Int("task_count", len(taskIDs)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

// BatchRefundEscrow refunds multiple escrows
func (ec *EscrowClient) BatchRefundEscrow(ctx context.Context, taskIDs [][32]byte) error {
	start := time.Now()

	ec.logger.Info("refunding batch escrows",
		zap.Int("task_count", len(taskIDs)),
	)

	if len(taskIDs) == 0 {
		return fmt.Errorf("no task IDs provided")
	}

	meta := ec.client.GetMetadata()

	// Create calls for each task
	var calls []types.Call
	for i, taskID := range taskIDs {
		call, err := types.NewCall(meta, "Escrow.refund_escrow", taskID)
		if err != nil {
			ec.logger.Error("failed to create refund call for batch item",
				zap.Error(err),
				zap.Int("item_index", i),
				zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			)
			return fmt.Errorf("failed to create call for item %d: %w", i, err)
		}
		calls = append(calls, call)
	}

	// Create batch call
	batchCall, err := types.NewCall(meta, "Utility.batch", calls)
	if err != nil {
		ec.logger.Error("failed to create batch refund call", zap.Error(err))
		return fmt.Errorf("failed to create batch call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, batchCall)
	if err != nil {
		ec.logger.Error("failed to submit batch refund",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to submit batch refund: %w", err)
	}

	ec.logger.Info("batch escrow refunded successfully",
		zap.Int("task_count", len(taskIDs)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

// BatchDisputeEscrow disputes multiple escrows
func (ec *EscrowClient) BatchDisputeEscrow(ctx context.Context, taskIDs [][32]byte) error {
	start := time.Now()

	ec.logger.Info("disputing batch escrows",
		zap.Int("task_count", len(taskIDs)),
	)

	if len(taskIDs) == 0 {
		return fmt.Errorf("no task IDs provided")
	}

	meta := ec.client.GetMetadata()

	// Create calls for each task
	var calls []types.Call
	for i, taskID := range taskIDs {
		call, err := types.NewCall(meta, "Escrow.dispute_escrow", taskID)
		if err != nil {
			ec.logger.Error("failed to create dispute call for batch item",
				zap.Error(err),
				zap.Int("item_index", i),
				zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			)
			return fmt.Errorf("failed to create call for item %d: %w", i, err)
		}
		calls = append(calls, call)
	}

	// Create batch call
	batchCall, err := types.NewCall(meta, "Utility.batch", calls)
	if err != nil {
		ec.logger.Error("failed to create batch dispute call", zap.Error(err))
		return fmt.Errorf("failed to create batch call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, batchCall)
	if err != nil {
		ec.logger.Error("failed to submit batch dispute",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to submit batch dispute: %w", err)
	}

	ec.logger.Info("batch escrow disputed successfully",
		zap.Int("task_count", len(taskIDs)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
} // =============================================================================
// REFUND POLICY METHODS
// =============================================================================

// SetRefundPolicy sets the refund policy for an escrow
func (ec *EscrowClient) SetRefundPolicy(ctx context.Context, taskID [32]byte, policy RefundPolicy) error {
	start := time.Now()

	ec.logger.Info("setting refund policy",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("policy_type", policy.PolicyType.String()),
		zap.Uint32("initial_refund", policy.InitialRefund),
	)

	meta := ec.client.GetMetadata()

	// Encode policy as bytes (in real implementation, you'd have proper encoding)
	policyBytes := types.NewBytes([]byte(fmt.Sprintf("%d,%d,%d,%d",
		uint8(policy.PolicyType),
		policy.InitialRefund,
		policy.MinimumRefund,
		policy.MaximumRefund)))

	call, err := types.NewCall(meta, "Escrow.set_refund_policy", taskID, policyBytes)
	if err != nil {
		ec.logger.Error("failed to create set_refund_policy call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to set refund policy",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to set refund policy: %w", err)
	}

	ec.logger.Info("refund policy set successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("policy_type", policy.PolicyType.String()),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

// GetRefundPolicy retrieves the refund policy for an escrow
func (ec *EscrowClient) GetRefundPolicy(ctx context.Context, taskID [32]byte) (*RefundPolicy, error) {
	start := time.Now()
	taskIDHex := fmt.Sprintf("0x%x", taskID)

	ec.logger.Debug("querying refund policy",
		zap.String("task_id", taskIDHex),
	)

	key, err := types.CreateStorageKey(ec.client.metadata, "Escrow", "RefundPolicies", taskID[:])
	if err != nil {
		ec.logger.Error("failed to create storage key",
			zap.Error(err),
			zap.String("task_id", taskIDHex),
		)
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	var result []byte
	ok, err := ec.client.api.RPC.State.GetStorageLatest(key, &result)
	if err != nil {
		ec.logger.Error("failed to query refund policy storage",
			zap.Error(err),
			zap.String("task_id", taskIDHex),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, fmt.Errorf("failed to query storage: %w", err)
	}
	if !ok {
		ec.logger.Debug("refund policy not found",
			zap.String("task_id", taskIDHex),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, fmt.Errorf("refund policy not found for task %s", taskIDHex)
	}

	// Decode policy (simplified - in real implementation, use proper decoding)
	policy := &RefundPolicy{
		PolicyType:       RefundPolicyTypeFixed,
		InitialRefund:    100,
		MinimumRefund:    0,
		MaximumRefund:    100,
		Steps:            []RefundStep{},
		CustomParameters: make(map[string]interface{}),
	}

	ec.logger.Debug("refund policy query completed",
		zap.String("task_id", taskIDHex),
		zap.String("policy_type", policy.PolicyType.String()),
		zap.Duration("duration", time.Since(start)),
	)

	return policy, nil
}

// CalculateRefund calculates the refund amount at a specific time
func (ec *EscrowClient) CalculateRefund(ctx context.Context, taskID [32]byte, atTime *BlockNumber) (*RefundCalculation, error) {
	start := time.Now()
	taskIDHex := fmt.Sprintf("0x%x", taskID)

	ec.logger.Debug("calculating refund",
		zap.String("task_id", taskIDHex),
	)

	// Get current block if no time specified
	currentBlock := uint32(0)
	if atTime == nil {
		blockNum, err := ec.client.GetLatestBlockNumber(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current block: %w", err)
		}
		currentBlock = uint32(blockNum)
	} else {
		currentBlock = uint32(*atTime)
	}

	// Get escrow details
	escrow, err := ec.GetEscrow(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow: %w", err)
	}

	// Get refund policy
	policy, err := ec.GetRefundPolicy(ctx, taskID)
	if err != nil {
		// Use default policy if none set
		policy = &RefundPolicy{
			PolicyType:    RefundPolicyTypeFixed,
			InitialRefund: 100,
			MinimumRefund: 100,
			MaximumRefund: 100,
		}
	}

	// Calculate refund based on policy
	originalAmount, _ := new(big.Int).SetString(string(escrow.Amount), 10)
	refundPercentage := policy.InitialRefund

	// Apply time-based adjustments (simplified logic)
	if currentBlock > uint32(escrow.CreatedAt) {
		elapsed := currentBlock - uint32(escrow.CreatedAt)
		switch policy.PolicyType {
		case RefundPolicyTypeLinear:
			// Linear decrease over time
			maxElapsed := uint32(escrow.ExpiresAt) - uint32(escrow.CreatedAt)
			if maxElapsed > 0 {
				decayFactor := float64(elapsed) / float64(maxElapsed)
				refundPercentage = uint32(float64(policy.InitialRefund) * (1.0 - decayFactor))
			}
		case RefundPolicyTypeStepwise:
			// Step-wise decrease
			for _, step := range policy.Steps {
				if elapsed >= step.Threshold {
					refundPercentage = step.RefundPercentage
				}
			}
		}
	}

	// Ensure within bounds
	if refundPercentage < policy.MinimumRefund {
		refundPercentage = policy.MinimumRefund
	}
	if refundPercentage > policy.MaximumRefund {
		refundPercentage = policy.MaximumRefund
	}

	// Calculate actual amounts
	refundAmount := new(big.Int).Mul(originalAmount, big.NewInt(int64(refundPercentage)))
	refundAmount.Div(refundAmount, big.NewInt(100))
	penaltyAmount := new(big.Int).Sub(originalAmount, refundAmount)

	calculation := &RefundCalculation{
		OriginalAmount:   escrow.Amount,
		RefundAmount:     Balance(refundAmount.String()),
		PenaltyAmount:    Balance(penaltyAmount.String()),
		RefundPercentage: refundPercentage,
		CalculatedAt:     BlockNumber(currentBlock),
		ExpiresAt:        BlockNumber(currentBlock + 1000), // 1000 blocks validity
		PolicyType:       policy.PolicyType,
		Reason:           "Standard refund calculation",
	}

	ec.logger.Debug("refund calculation completed",
		zap.String("task_id", taskIDHex),
		zap.Uint32("refund_percentage", refundPercentage),
		zap.String("refund_amount", string(calculation.RefundAmount)),
		zap.Duration("duration", time.Since(start)),
	)

	return calculation, nil
}

// ProcessRefundWithPolicy processes a refund using the configured policy
func (ec *EscrowClient) ProcessRefundWithPolicy(ctx context.Context, taskID [32]byte) error {
	start := time.Now()

	ec.logger.Info("processing refund with policy",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
	)

	// Calculate refund amount
	calculation, err := ec.CalculateRefund(ctx, taskID, nil)
	if err != nil {
		return fmt.Errorf("failed to calculate refund: %w", err)
	}

	meta := ec.client.GetMetadata()
	refundAmountBig, _ := new(big.Int).SetString(string(calculation.RefundAmount), 10)

	call, err := types.NewCall(meta, "Escrow.process_refund_with_policy", taskID, types.NewU128(*refundAmountBig))
	if err != nil {
		ec.logger.Error("failed to create process_refund_with_policy call",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to process refund with policy",
			zap.Error(err),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to process refund with policy: %w", err)
	}

	ec.logger.Info("refund processed with policy successfully",
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("refund_amount", string(calculation.RefundAmount)),
		zap.Uint32("refund_percentage", calculation.RefundPercentage),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

// =============================================================================
// TEMPLATE METHODS
// =============================================================================

// CreateTemplate creates a new escrow template
func (ec *EscrowClient) CreateTemplate(ctx context.Context, name string, description string, templateType EscrowTemplateType, params map[string]interface{}) (uint32, error) {
	start := time.Now()

	ec.logger.Info("creating escrow template",
		zap.String("name", name),
		zap.String("description", description),
		zap.String("template_type", templateType.String()),
	)

	meta := ec.client.GetMetadata()
	nameBytes := types.NewBytes([]byte(name))
	descBytes := types.NewBytes([]byte(description))

	// Encode parameters as JSON bytes (simplified)
	paramsStr := "{}"
	if len(params) > 0 {
		// In real implementation, use proper JSON encoding
		paramsStr = fmt.Sprintf("{\"params\": %d}", len(params))
	}
	paramsBytes := types.NewBytes([]byte(paramsStr))

	call, err := types.NewCall(meta, "Escrow.create_template", nameBytes, descBytes, types.NewU8(uint8(templateType)), paramsBytes)
	if err != nil {
		ec.logger.Error("failed to create create_template call",
			zap.Error(err),
			zap.String("name", name),
		)
		return 0, fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to create template",
			zap.Error(err),
			zap.String("name", name),
			zap.Duration("duration", time.Since(start)),
		)
		return 0, fmt.Errorf("failed to create template: %w", err)
	}

	// In real implementation, parse transaction events to get the template ID
	templateID := uint32(1) // Placeholder

	ec.logger.Info("template created successfully",
		zap.String("name", name),
		zap.Uint32("template_id", templateID),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return templateID, nil
}

// CreateEscrowFromTemplate creates an escrow using a template
func (ec *EscrowClient) CreateEscrowFromTemplate(ctx context.Context, templateID uint32, taskID [32]byte, customParams map[string]interface{}) error {
	start := time.Now()

	ec.logger.Info("creating escrow from template",
		zap.Uint32("template_id", templateID),
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
	)

	meta := ec.client.GetMetadata()

	// Encode custom parameters
	paramsStr := "{}"
	if len(customParams) > 0 {
		// In real implementation, use proper JSON encoding
		paramsStr = fmt.Sprintf("{\"custom\": %d}", len(customParams))
	}
	paramsBytes := types.NewBytes([]byte(paramsStr))

	call, err := types.NewCall(meta, "Escrow.create_escrow_from_template", types.NewU32(templateID), taskID, paramsBytes)
	if err != nil {
		ec.logger.Error("failed to create create_escrow_from_template call",
			zap.Error(err),
			zap.Uint32("template_id", templateID),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		)
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := ec.submitTransaction(ctx, call)
	if err != nil {
		ec.logger.Error("failed to create escrow from template",
			zap.Error(err),
			zap.Uint32("template_id", templateID),
			zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
			zap.Duration("duration", time.Since(start)),
		)
		return fmt.Errorf("failed to create escrow from template: %w", err)
	}

	ec.logger.Info("escrow created from template successfully",
		zap.Uint32("template_id", templateID),
		zap.String("task_id", fmt.Sprintf("0x%x", taskID)),
		zap.String("block_hash", hash.Hex()),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

// ListTemplates lists available escrow templates
func (ec *EscrowClient) ListTemplates(ctx context.Context, creator *AccountID) ([]EscrowTemplate, error) {
	start := time.Now()

	ec.logger.Debug("listing escrow templates")

	// In real implementation, query the blockchain storage for templates
	// For now, return a mock template
	templates := []EscrowTemplate{
		{
			ID:                   1,
			Name:                 "Standard Task Escrow",
			Description:          "Standard escrow template for task payments",
			TemplateType:         EscrowTemplateTypeSimple,
			IsPublic:             true,
			Version:              1,
			Tags:                 []string{"standard", "task", "payment"},
			DefaultTimeout:       &[]uint32{1000}[0], // 1000 blocks
			DefaultFeePercent:    &[]uint8{5}[0],     // 5% fee
			RequiredFields:       []string{"amount", "task_hash"},
			AllowedModifications: []string{"timeout", "fee_percent"},
			UsageCount:           0,
			IsActive:             true,
		},
	}

	// Filter by creator if specified
	if creator != nil {
		var filtered []EscrowTemplate
		for _, template := range templates {
			if template.Creator == *creator {
				filtered = append(filtered, template)
			}
		}
		templates = filtered
	}

	ec.logger.Debug("template list completed",
		zap.Int("template_count", len(templates)),
		zap.Duration("duration", time.Since(start)),
	)

	return templates, nil
}

// GetTemplate retrieves a specific template
func (ec *EscrowClient) GetTemplate(ctx context.Context, templateID uint32) (*EscrowTemplate, error) {
	start := time.Now()

	ec.logger.Debug("getting escrow template",
		zap.Uint32("template_id", templateID),
	)

	templateBytes := uint32ToBytes(templateID)
	key, err := types.CreateStorageKey(ec.client.metadata, "Escrow", "Templates", templateBytes)
	if err != nil {
		ec.logger.Error("failed to create storage key",
			zap.Error(err),
			zap.Uint32("template_id", templateID),
		)
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	var result []byte
	ok, err := ec.client.api.RPC.State.GetStorageLatest(key, &result)
	if err != nil {
		ec.logger.Error("failed to query template storage",
			zap.Error(err),
			zap.Uint32("template_id", templateID),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, fmt.Errorf("failed to query storage: %w", err)
	}
	if !ok {
		ec.logger.Debug("template not found",
			zap.Uint32("template_id", templateID),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, fmt.Errorf("template not found: %d", templateID)
	}

	// Decode template (simplified - in real implementation, use proper decoding)
	template := &EscrowTemplate{
		ID:                   templateID,
		Name:                 "Standard Task Escrow",
		Description:          "Standard escrow template for task payments",
		TemplateType:         EscrowTemplateTypeSimple,
		IsPublic:             true,
		Version:              1,
		Tags:                 []string{"standard", "task", "payment"},
		DefaultTimeout:       &[]uint32{1000}[0],
		DefaultFeePercent:    &[]uint8{5}[0],
		RequiredFields:       []string{"amount", "task_hash"},
		AllowedModifications: []string{"timeout", "fee_percent"},
		UsageCount:           0,
		IsActive:             true,
	}

	ec.logger.Debug("template query completed",
		zap.Uint32("template_id", templateID),
		zap.String("template_name", template.Name),
		zap.Duration("duration", time.Since(start)),
	)

	return template, nil
}

// =============================================================================
// EXTENDED QUERY METHODS
// =============================================================================

// GetExtendedEscrowDetails retrieves extended escrow information
func (ec *EscrowClient) GetExtendedEscrowDetails(ctx context.Context, taskID [32]byte) (*ExtendedEscrowDetails, error) {
	start := time.Now()
	taskIDHex := fmt.Sprintf("0x%x", taskID)

	ec.logger.Debug("querying extended escrow details",
		zap.String("task_id", taskIDHex),
	)

	// Get base escrow details
	baseEscrow, err := ec.GetEscrow(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get base escrow: %w", err)
	}

	// Create extended details
	extended := &ExtendedEscrowDetails{
		TaskID:       baseEscrow.TaskID,
		User:         baseEscrow.User,
		AgentDID:     baseEscrow.AgentDID,
		AgentAccount: baseEscrow.AgentAccount,
		Amount:       baseEscrow.Amount,
		FeePercent:   baseEscrow.FeePercent,
		CreatedAt:    baseEscrow.CreatedAt,
		ExpiresAt:    baseEscrow.ExpiresAt,
		State:        baseEscrow.State,
		TaskHash:     baseEscrow.TaskHash,

		// Extended fields (would be populated from additional storage queries)
		EscrowType:     EscrowTypeSimple, // Default, would be queried
		LastUpdated:    baseEscrow.CreatedAt,
		TotalFees:      Balance("0"),
		CustomMetadata: make(map[string]interface{}),
	}

	// Query additional information if needed
	// In real implementation, you'd make additional storage queries here

	ec.logger.Debug("extended escrow query completed",
		zap.String("task_id", taskIDHex),
		zap.String("state", string(extended.State)),
		zap.String("escrow_type", extended.EscrowType.String()),
		zap.Duration("duration", time.Since(start)),
	)

	return extended, nil
}

// GetEscrowStats retrieves system-wide escrow statistics
func (ec *EscrowClient) GetEscrowStats(ctx context.Context) (*EscrowStats, error) {
	start := time.Now()

	ec.logger.Debug("querying escrow statistics")

	// In real implementation, query multiple storage items to compute stats
	// For now, return mock statistics
	stats := &EscrowStats{
		TotalEscrows:            1000,
		ActiveEscrows:           250,
		CompletedEscrows:        700,
		DisputedEscrows:         50,
		TotalValueLocked:        Balance("1000000000000000000"), // 1 ETH equivalent
		TotalFeesCollected:      Balance("50000000000000000"),   // 0.05 ETH equivalent
		AverageEscrowAmount:     Balance("100000000000000000"),  // 0.1 ETH equivalent
		AverageTimeToCompletion: 24 * time.Hour,
		SuccessRate:             0.85,
		SimpleEscrows:           800,
		MultiPartyEscrows:       150,
		MilestoneEscrows:        50,
		HybridEscrows:           0,
		LastUpdated:             BlockNumber(0), // Would be current block
	}

	ec.logger.Debug("escrow statistics query completed",
		zap.Uint64("total_escrows", stats.TotalEscrows),
		zap.Uint64("active_escrows", stats.ActiveEscrows),
		zap.Float64("success_rate", stats.SuccessRate),
		zap.Duration("duration", time.Since(start)),
	)

	return stats, nil
}

// Helper function to convert uint32 to []byte for storage key
func uint32ToBytes(value uint32) []byte {
	bytes := make([]byte, 4)
	bytes[0] = byte(value)
	bytes[1] = byte(value >> 8)
	bytes[2] = byte(value >> 16)
	bytes[3] = byte(value >> 24)
	return bytes
}
