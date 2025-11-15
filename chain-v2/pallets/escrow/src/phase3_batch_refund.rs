//! # Sprint 8 Phase 3: Batch Operations & Advanced Refund Policies
//!
//! This module implements batch escrow operations and advanced refund policy management
//! for enterprise scenarios and power users.

use codec::DecodeWithMemTracking;
use frame_support::pallet_prelude::*;
use frame_system::pallet_prelude::*;

use super::*;

/// Advanced refund policy types for Phase 3
#[derive(
    Clone,
    Encode,
    Decode,
    DecodeWithMemTracking,
    Eq,
    PartialEq,
    RuntimeDebug,
    TypeInfo,
    MaxEncodedLen,
)]
pub enum RefundPolicyType<T: Config> {
    /// Full refund before deadline, partial after
    TimeBased {
        full_refund_deadline: BlockNumberFor<T>,
        partial_refund_percentage: u8,
    },
    /// Percentage decreases over time stages
    Graduated {
        stages: BoundedVec<(BlockNumberFor<T>, u8), ConstU32<10>>,
    },
    /// Fixed fee deducted from refund
    CancellationFee { fee_amount: BalanceOf<T> },
    /// No refunds after work started
    NoRefund {
        work_start_deadline: BlockNumberFor<T>,
    },
    /// Based on milestone completion
    Conditional {
        milestones_completed: u8,
        refund_percentages: BoundedVec<u8, ConstU32<10>>,
    },
    /// Arbitrator decides refund amount
    DisputeBased,
    /// Standard policy - full refund if not accepted
    Standard,
}

/// Refund policy for an escrow
#[derive(
    Clone,
    Encode,
    Decode,
    DecodeWithMemTracking,
    Eq,
    PartialEq,
    RuntimeDebug,
    TypeInfo,
    MaxEncodedLen,
)]
pub struct RefundPolicy<T: Config> {
    pub policy_type: RefundPolicyType<T>,
    pub can_override: bool,
    pub override_authority: Option<T::AccountId>,
    pub created_at: BlockNumberFor<T>,
}

/// Batch operation request for creating multiple escrows
#[derive(Clone, Encode, Decode, DecodeWithMemTracking, Eq, PartialEq, RuntimeDebug, TypeInfo)]
pub struct BatchCreateEscrowRequest<T: Config> {
    pub task_id: [u8; 32],
    pub amount: BalanceOf<T>,
    pub task_hash: [u8; 32],
    pub timeout_blocks: Option<BlockNumberFor<T>>,
    pub refund_policy: Option<RefundPolicy<T>>,
}

/// Batch operation result
#[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo)]
pub struct BatchOperationResult {
    pub successful_operations: u32,
    pub failed_operations: u32,
    pub first_failure_index: Option<u32>,
    pub total_amount_processed: Option<u128>,
}

/// Phase 3 specific storage items - these are defined in the main pallet module
/// Phase 3 specific events
pub enum Phase3Event<T: Config> {
    // Batch operation events
    BatchOperationCompleted {
        batch_id: [u8; 32],
        operation_type: BoundedVec<u8, ConstU32<32>>, // "create", "release", "refund", "dispute"
        result: BatchOperationResult,
    },
    BatchOperationFailed {
        batch_id: [u8; 32],
        operation_type: BoundedVec<u8, ConstU32<32>>,
        failure_index: u32,
        error: BoundedVec<u8, ConstU32<128>>,
    },

    // Refund policy events
    RefundPolicySet {
        task_id: [u8; 32],
        policy_type: BoundedVec<u8, ConstU32<32>>, // policy type name
        can_override: bool,
    },
    RefundPolicyUpdated {
        task_id: [u8; 32],
        old_policy: BoundedVec<u8, ConstU32<32>>,
        new_policy: BoundedVec<u8, ConstU32<32>>,
        updated_by: T::AccountId,
    },
    RefundPolicyOverridden {
        task_id: [u8; 32],
        original_amount: BalanceOf<T>,
        override_amount: BalanceOf<T>,
        overridden_by: T::AccountId,
    },
    RefundAmountCalculated {
        task_id: [u8; 32],
        policy_type: BoundedVec<u8, ConstU32<32>>,
        original_amount: BalanceOf<T>,
        refund_amount: BalanceOf<T>,
    },
}

/// Phase 3 specific errors
pub enum Phase3Error {
    // Batch operation errors
    BatchSizeExceeded,
    BatchAlreadyInProgress,
    BatchOperationFailed,
    InvalidBatchSize,
    InsufficientBalanceForBatch,

    // Refund policy errors
    InvalidRefundPolicy,
    RefundPolicyNotFound,
    CannotOverridePolicy,
    NotAuthorizedToOverride,
    InvalidRefundPercentage,
    RefundPolicyExpired,
    GraduatedStagesInvalid,
    ConditionalMilestonesInvalid,
    TimePolicyInvalid,
}

/// Phase 3 constants for batch operations and refund policies
pub const MAX_BATCH_SIZE: u32 = 50;
pub const MIN_REFUND_PERCENTAGE: u8 = 1;
pub const MAX_REFUND_PERCENTAGE: u8 = 100;
