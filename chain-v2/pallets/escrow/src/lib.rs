//! # Pallet Escrow (Trustless Payment Settlement)
//!
//! This pallet implements on-chain escrow for task-based payments in the Ainur marketplace.
//! It provides trustless fund locking, release, and dispute resolution.

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

// Sprint 8 Phase 2: Escrow Template System
pub mod templates;

// Sprint 8 Phase 3: Batch Operations & Advanced Refund Policies
pub mod phase3_batch_refund;

#[frame_support::pallet]
pub mod pallet {
    use crate::phase3_batch_refund;
    use crate::templates;
    use codec::DecodeWithMemTracking;
    use frame_support::pallet_prelude::*;
    use frame_support::traits::{Currency, ExistenceRequirement, ReservableCurrency};
    use frame_system::pallet_prelude::*;
    use sp_runtime::traits::{
        CheckedDiv, CheckedMul, CheckedSub, SaturatedConversion, Saturating, Zero,
    };
    use sp_std::vec::Vec;

    /// Type alias for balance (AINU tokens)
    pub type BalanceOf<T> =
        <<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

    /// Escrow state machine
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    pub enum EscrowState {
        Pending,
        Accepted,
        Completed,
        Refunded,
        Disputed,
    }

    /// Participant in a multi-party escrow
    #[derive(Clone, Encode, Decode, Eq, PartialEq, Debug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct EscrowParticipant<T: Config> {
        pub account: T::AccountId,
        pub role: ParticipantRole,
        pub amount: BalanceOf<T>,
        pub approved: bool,
    }

    /// Participant role in escrow
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
    pub enum ParticipantRole {
        Payer,
        Payee,
        Arbiter,
    }

    /// Milestone for conditional escrow
    #[derive(Clone, Encode, Decode, Eq, PartialEq, Debug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct Milestone<T: Config> {
        pub id: u32,
        pub description: BoundedVec<u8, ConstU32<256>>,
        pub amount: BalanceOf<T>,
        pub completed: bool,
        pub approved_by: BoundedVec<T::AccountId, ConstU32<10>>,
        pub required_approvals: u32,
    }

    /// Escrow details stored on-chain
    #[derive(Encode, Decode, Eq, PartialEq, Debug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct EscrowDetails<T: Config> {
        pub task_id: [u8; 32],
        pub user: T::AccountId,
        pub agent_did: Option<BoundedVec<u8, T::MaxDidLength>>,
        pub agent_account: Option<T::AccountId>,
        pub amount: BalanceOf<T>,
        pub fee_percent: u8,
        pub created_at: BlockNumberFor<T>,
        pub expires_at: BlockNumberFor<T>,
        pub state: EscrowState,
        pub task_hash: [u8; 32],
        // Multi-party escrow fields
        pub participants: BoundedVec<EscrowParticipant<T>, ConstU32<10>>,
        pub is_multi_party: bool,
        // Milestone-based escrow fields
        pub milestones: BoundedVec<Milestone<T>, ConstU32<20>>,
        pub is_milestone_based: bool,
        pub next_milestone_id: u32,
    }

    impl<T: Config> Clone for EscrowDetails<T> {
        fn clone(&self) -> Self {
            Self {
                task_id: self.task_id,
                user: self.user.clone(),
                agent_did: self.agent_did.clone(),
                agent_account: self.agent_account.clone(),
                amount: self.amount,
                fee_percent: self.fee_percent,
                created_at: self.created_at,
                expires_at: self.expires_at,
                state: self.state.clone(),
                task_hash: self.task_hash,
                participants: self.participants.clone(),
                is_multi_party: self.is_multi_party,
                milestones: self.milestones.clone(),
                is_milestone_based: self.is_milestone_based,
                next_milestone_id: self.next_milestone_id,
            }
        }
    }

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    #[pallet::config]
    pub trait Config:
        frame_system::Config
        + pallet_did::Config
        + pallet_registry::Config
        + core::fmt::Debug
        + TypeInfo
    {
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
        type Currency: Currency<Self::AccountId> + ReservableCurrency<Self::AccountId>;

        #[pallet::constant]
        type DefaultTimeout: Get<BlockNumberFor<Self>>;

        #[pallet::constant]
        type ProtocolFeeAccount: Get<Self::AccountId>;

        #[pallet::constant]
        type MaxEscrowAmount: Get<BalanceOf<Self>>;

        #[pallet::constant]
        type MaxParticipants: Get<u32>;

        #[pallet::constant]
        type MaxMilestones: Get<u32>;

        /// Phase 3: Maximum batch size for operations
        #[pallet::constant]
        type MaxBatchSize: Get<u32>;
    }

    #[pallet::storage]
    #[pallet::getter(fn escrows)]
    pub type Escrows<T: Config> =
        StorageMap<_, Blake2_128Concat, [u8; 32], EscrowDetails<T>, OptionQuery>;

    #[pallet::storage]
    #[pallet::getter(fn user_escrows)]
    pub type UserEscrows<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        T::AccountId,
        BoundedVec<[u8; 32], ConstU32<1000>>,
        ValueQuery,
    >;

    #[pallet::storage]
    #[pallet::getter(fn agent_escrows)]
    pub type AgentEscrows<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        BoundedVec<u8, T::MaxDidLength>,
        BoundedVec<[u8; 32], ConstU32<1000>>,
        ValueQuery,
    >;

    /// Participant escrow tracking
    #[pallet::storage]
    #[pallet::getter(fn participant_escrows)]
    pub type ParticipantEscrows<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        T::AccountId,
        BoundedVec<[u8; 32], ConstU32<1000>>,
        ValueQuery,
    >;

    /// Milestone approval tracking
    #[pallet::storage]
    #[pallet::getter(fn milestone_approvals)]
    pub type MilestoneApprovals<T: Config> = StorageDoubleMap<
        _,
        Blake2_128Concat,
        [u8; 32], // task_id
        Blake2_128Concat,
        u32,                                    // milestone_id
        BoundedVec<T::AccountId, ConstU32<10>>, // approved_by
        ValueQuery,
    >;

    /// Phase 3: Storage for escrow refund policies
    #[pallet::storage]
    #[pallet::getter(fn escrow_refund_policies)]
    pub type EscrowRefundPolicies<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        [u8; 32], // task_id
        phase3_batch_refund::RefundPolicy<T>,
        OptionQuery,
    >;

    /// Phase 3: Batch operations in progress (to prevent double execution)
    #[pallet::storage]
    #[pallet::getter(fn batch_operations_in_progress)]
    pub type BatchOperationsInProgress<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        [u8; 32],          // batch_id
        BlockNumberFor<T>, // started_at_block
        OptionQuery,
    >;

    /// Phase 3: Batch operation counters
    #[pallet::storage]
    #[pallet::getter(fn batch_operation_counters)]
    pub type BatchOperationCounters<T: Config> = StorageValue<
        _,
        (u64, u64), // (total_batches, total_operations)
        ValueQuery,
    >;

    /// Phase 2: Storage for escrow templates
    #[pallet::storage]
    #[pallet::getter(fn escrow_templates)]
    pub type EscrowTemplates<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        u32, // template_id
        templates::EscrowTemplate<T>,
        OptionQuery,
    >;

    /// Phase 2: Counter for next available template ID
    #[pallet::storage]
    #[pallet::getter(fn next_template_id)]
    pub type NextTemplateId<T: Config> = StorageValue<_, u32, ValueQuery>;

    /// Phase 2: Templates indexed by creator
    #[pallet::storage]
    #[pallet::getter(fn templates_by_creator)]
    pub type TemplatesByCreator<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        T::AccountId,                   // creator
        BoundedVec<u32, ConstU32<100>>, // template_ids
        ValueQuery,
    >;

    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        EscrowCreated {
            task_id: [u8; 32],
            user: T::AccountId,
            amount: BalanceOf<T>,
        },
        TaskAccepted {
            task_id: [u8; 32],
            agent_did: Vec<u8>,
            agent_account: T::AccountId,
        },
        PaymentReleased {
            task_id: [u8; 32],
            agent: T::AccountId,
            amount: BalanceOf<T>,
            fee: BalanceOf<T>,
        },
        EscrowRefunded {
            task_id: [u8; 32],
            user: T::AccountId,
            amount: BalanceOf<T>,
        },
        DisputeRaised {
            task_id: [u8; 32],
            raised_by: T::AccountId,
        },
        // Multi-party escrow events
        ParticipantAdded {
            task_id: [u8; 32],
            participant: T::AccountId,
            role: ParticipantRole,
            amount: BalanceOf<T>,
        },
        ParticipantRemoved {
            task_id: [u8; 32],
            participant: T::AccountId,
        },
        MultiPartyRelease {
            task_id: [u8; 32],
            total_amount: BalanceOf<T>,
            participants_count: u32,
        },
        // Milestone-based escrow events
        MilestoneAdded {
            task_id: [u8; 32],
            milestone_id: u32,
            amount: BalanceOf<T>,
            required_approvals: u32,
        },
        MilestoneCompleted {
            task_id: [u8; 32],
            milestone_id: u32,
            completed_by: T::AccountId,
        },
        MilestoneApproved {
            task_id: [u8; 32],
            milestone_id: u32,
            approved_by: T::AccountId,
        },
        MilestonePaid {
            task_id: [u8; 32],
            milestone_id: u32,
            amount: BalanceOf<T>,
            recipient: T::AccountId,
        },

        // Phase 3: Batch operation events
        BatchOperationCompleted {
            batch_id: [u8; 32],
            operation_type: BoundedVec<u8, ConstU32<32>>,
            successful_operations: u32,
            failed_operations: u32,
            total_amount_processed: BalanceOf<T>,
        },
        BatchOperationFailed {
            batch_id: [u8; 32],
            operation_type: BoundedVec<u8, ConstU32<32>>,
            failure_index: u32,
            error_message: BoundedVec<u8, ConstU32<128>>,
        },

        // Phase 3: Refund policy events
        RefundPolicySet {
            task_id: [u8; 32],
            policy_type: BoundedVec<u8, ConstU32<32>>,
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

        // Phase 2: Template system events
        TemplateCreated {
            template_id: u32,
            template_type: BoundedVec<u8, ConstU32<32>>,
            created_by: T::AccountId,
        },
        TemplateUpdated {
            template_id: u32,
            updated_by: T::AccountId,
        },
        TemplateDeactivated {
            template_id: u32,
            deactivated_by: T::AccountId,
        },
        EscrowCreatedFromTemplate {
            task_id: [u8; 32],
            template_id: u32,
            user: T::AccountId,
            amount: BalanceOf<T>,
        },
    }

    #[pallet::error]
    pub enum Error<T> {
        EscrowAlreadyExists,
        EscrowNotFound,
        InsufficientBalance,
        AmountTooLarge,
        InvalidEscrowState,
        NotEscrowCreator,
        NotAssignedAgent,
        InvalidAgentDid,
        EscrowExpired,
        EscrowNotExpired,
        TooManyUserEscrows,
        TooManyAgentEscrows,
        ArithmeticOverflow,
        // Multi-party escrow errors
        ParticipantNotFound,
        ParticipantAlreadyExists,
        TooManyParticipants,
        InvalidParticipantRole,
        NotParticipant,
        InsufficientApprovals,
        ParticipantNotApproved,
        // Milestone-based escrow errors
        MilestoneNotFound,
        MilestoneAlreadyCompleted,
        MilestoneNotCompleted,
        TooManyMilestones,
        InvalidMilestone,
        AlreadyApproved,
        NotAuthorizedToApprove,
        MilestoneAmountMismatch,

        // Phase 3: Batch operation errors
        BatchSizeExceeded,
        BatchAlreadyInProgress,
        BatchOperationFailed,
        InvalidBatchSize,
        InsufficientBalanceForBatch,

        // Phase 3: Refund policy errors
        InvalidRefundPolicy,
        RefundPolicyNotFound,
        CannotOverridePolicy,
        NotAuthorizedToOverride,
        InvalidRefundPercentage,
        RefundPolicyExpired,
        GraduatedStagesInvalid,
        ConditionalMilestonesInvalid,
        TimePolicyInvalid,

        // Phase 2: Template system errors
        TemplateNotFound,
        TemplateInactive,
        TemplateNameTooLong,
        TemplateDescriptionTooLong,
        InvalidTemplateParams,
        TooManyTemplates,
        InvalidFeePercentage,
        InvalidAmountRange,
        NotTemplateCreator,
        CannotUpdateBuiltinTemplate,
    }

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        #[pallet::call_index(0)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn create_escrow(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            amount: BalanceOf<T>,
            task_hash: [u8; 32],
            timeout_blocks: Option<BlockNumberFor<T>>,
        ) -> DispatchResult {
            let user = ensure_signed(origin)?;

            ensure!(amount > Zero::zero(), Error::<T>::InsufficientBalance);
            ensure!(
                amount <= T::MaxEscrowAmount::get(),
                Error::<T>::AmountTooLarge
            );
            ensure!(
                !Escrows::<T>::contains_key(task_id),
                Error::<T>::EscrowAlreadyExists
            );

            T::Currency::reserve(&user, amount).map_err(|_| Error::<T>::InsufficientBalance)?;

            let current_block = <frame_system::Pallet<T>>::block_number();
            let timeout = timeout_blocks.unwrap_or_else(T::DefaultTimeout::get);
            let expires_at = current_block + timeout;

            let escrow = EscrowDetails {
                task_id,
                user: user.clone(),
                agent_did: None,
                agent_account: None,
                amount,
                fee_percent: 5,
                created_at: current_block,
                expires_at,
                state: EscrowState::Pending,
                task_hash,
                participants: BoundedVec::new(),
                is_multi_party: false,
                milestones: BoundedVec::new(),
                is_milestone_based: false,
                next_milestone_id: 0,
            };

            Escrows::<T>::insert(task_id, escrow);

            UserEscrows::<T>::try_mutate(&user, |tasks| {
                tasks
                    .try_push(task_id)
                    .map_err(|_| Error::<T>::TooManyUserEscrows)
            })?;

            Self::deposit_event(Event::EscrowCreated {
                task_id,
                user,
                amount,
            });

            Ok(())
        }

        #[pallet::call_index(1)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn accept_task(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            agent_did: Vec<u8>,
        ) -> DispatchResult {
            let agent_account = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(
                escrow.state == EscrowState::Pending,
                Error::<T>::InvalidEscrowState
            );

            let current_block = <frame_system::Pallet<T>>::block_number();
            ensure!(current_block < escrow.expires_at, Error::<T>::EscrowExpired);

            let bounded_did: BoundedVec<u8, T::MaxDidLength> = agent_did
                .clone()
                .try_into()
                .map_err(|_| Error::<T>::InvalidAgentDid)?;

            ensure!(
                pallet_did::Pallet::<T>::is_did_active(&agent_did),
                Error::<T>::InvalidAgentDid
            );

            escrow.state = EscrowState::Accepted;
            escrow.agent_did = Some(bounded_did.clone());
            escrow.agent_account = Some(agent_account.clone());
            Escrows::<T>::insert(task_id, escrow);

            AgentEscrows::<T>::try_mutate(&bounded_did, |tasks| {
                tasks
                    .try_push(task_id)
                    .map_err(|_| Error::<T>::TooManyAgentEscrows)
            })?;

            Self::deposit_event(Event::TaskAccepted {
                task_id,
                agent_did,
                agent_account,
            });

            Ok(())
        }

        #[pallet::call_index(2)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn release_payment(origin: OriginFor<T>, task_id: [u8; 32]) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(escrow.user == caller, Error::<T>::NotEscrowCreator);
            ensure!(
                escrow.state == EscrowState::Accepted,
                Error::<T>::InvalidEscrowState
            );

            let agent = escrow
                .agent_account
                .clone()
                .ok_or(Error::<T>::InvalidEscrowState)?;

            let fee_amount = Self::calculate_fee(escrow.amount, escrow.fee_percent)?;
            let net_amount = escrow
                .amount
                .checked_sub(&fee_amount)
                .ok_or(Error::<T>::ArithmeticOverflow)?;

            T::Currency::unreserve(&escrow.user, escrow.amount);

            T::Currency::transfer(
                &escrow.user,
                &agent,
                net_amount,
                ExistenceRequirement::KeepAlive,
            )?;

            T::Currency::transfer(
                &escrow.user,
                &T::ProtocolFeeAccount::get(),
                fee_amount,
                ExistenceRequirement::AllowDeath,
            )?;

            escrow.state = EscrowState::Completed;
            Escrows::<T>::insert(task_id, escrow);

            Self::deposit_event(Event::PaymentReleased {
                task_id,
                agent,
                amount: net_amount,
                fee: fee_amount,
            });

            Ok(())
        }

        #[pallet::call_index(3)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn refund_escrow(origin: OriginFor<T>, task_id: [u8; 32]) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(
                escrow.state == EscrowState::Pending || escrow.state == EscrowState::Accepted,
                Error::<T>::InvalidEscrowState
            );

            let current_block = <frame_system::Pallet<T>>::block_number();
            let is_expired = current_block >= escrow.expires_at;

            if escrow.state == EscrowState::Pending {
                ensure!(escrow.user == caller, Error::<T>::NotEscrowCreator);
            } else if escrow.state == EscrowState::Accepted {
                ensure!(is_expired, Error::<T>::EscrowNotExpired);
            }

            T::Currency::unreserve(&escrow.user, escrow.amount);

            escrow.state = EscrowState::Refunded;
            Escrows::<T>::insert(task_id, escrow.clone());

            Self::deposit_event(Event::EscrowRefunded {
                task_id,
                user: escrow.user,
                amount: escrow.amount,
            });

            Ok(())
        }

        #[pallet::call_index(4)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn dispute_escrow(origin: OriginFor<T>, task_id: [u8; 32]) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(
                escrow.state == EscrowState::Accepted,
                Error::<T>::InvalidEscrowState
            );

            let is_user = escrow.user == caller;
            let is_agent = escrow.agent_account.as_ref() == Some(&caller);
            ensure!(is_user || is_agent, Error::<T>::NotEscrowCreator);

            escrow.state = EscrowState::Disputed;
            Escrows::<T>::insert(task_id, escrow);

            Self::deposit_event(Event::DisputeRaised {
                task_id,
                raised_by: caller,
            });

            Ok(())
        }

        /// Add a participant to a multi-party escrow
        #[pallet::call_index(5)]
        #[pallet::weight(Weight::from_parts(15_000, 0))]
        pub fn add_participant(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            participant: T::AccountId,
            role: ParticipantRole,
            amount: BalanceOf<T>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(escrow.user == caller, Error::<T>::NotEscrowCreator);
            ensure!(
                escrow.state == EscrowState::Pending,
                Error::<T>::InvalidEscrowState
            );
            ensure!(amount > Zero::zero(), Error::<T>::InsufficientBalance);

            // Check if participant already exists
            let participant_exists = escrow.participants.iter().any(|p| p.account == participant);
            ensure!(!participant_exists, Error::<T>::ParticipantAlreadyExists);

            // Check participant limit
            ensure!(
                escrow.participants.len() < T::MaxParticipants::get() as usize,
                Error::<T>::TooManyParticipants
            );

            // Reserve funds for payers
            if role == ParticipantRole::Payer {
                T::Currency::reserve(&participant, amount)
                    .map_err(|_| Error::<T>::InsufficientBalance)?;
            }

            let new_participant = EscrowParticipant {
                account: participant.clone(),
                role: role.clone(),
                amount,
                approved: false,
            };

            escrow
                .participants
                .try_push(new_participant)
                .map_err(|_| Error::<T>::TooManyParticipants)?;
            escrow.is_multi_party = true;

            Escrows::<T>::insert(task_id, escrow);

            // Add to participant tracking
            ParticipantEscrows::<T>::try_mutate(&participant, |tasks| {
                tasks
                    .try_push(task_id)
                    .map_err(|_| Error::<T>::TooManyUserEscrows)
            })?;

            Self::deposit_event(Event::ParticipantAdded {
                task_id,
                participant,
                role,
                amount,
            });

            Ok(())
        }

        /// Remove a participant from a multi-party escrow (with consent)
        #[pallet::call_index(6)]
        #[pallet::weight(Weight::from_parts(15_000, 0))]
        pub fn remove_participant(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            participant: T::AccountId,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(
                escrow.state == EscrowState::Pending,
                Error::<T>::InvalidEscrowState
            );

            // Only escrow creator or the participant themselves can remove
            ensure!(
                escrow.user == caller || participant == caller,
                Error::<T>::NotEscrowCreator
            );

            // Find and remove participant
            let participant_index = escrow
                .participants
                .iter()
                .position(|p| p.account == participant)
                .ok_or(Error::<T>::ParticipantNotFound)?;

            let removed_participant = escrow.participants.remove(participant_index);

            // Unreserve funds if it was a payer
            if removed_participant.role == ParticipantRole::Payer {
                T::Currency::unreserve(&participant, removed_participant.amount);
            }

            // Update multi-party status
            if escrow.participants.is_empty() {
                escrow.is_multi_party = false;
            }

            Escrows::<T>::insert(task_id, escrow);

            Self::deposit_event(Event::ParticipantRemoved {
                task_id,
                participant,
            });

            Ok(())
        }

        /// Add a milestone to an escrow
        #[pallet::call_index(7)]
        #[pallet::weight(Weight::from_parts(15_000, 0))]
        pub fn add_milestone(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            description: Vec<u8>,
            amount: BalanceOf<T>,
            required_approvals: u32,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(escrow.user == caller, Error::<T>::NotEscrowCreator);
            ensure!(
                escrow.state == EscrowState::Pending,
                Error::<T>::InvalidEscrowState
            );
            ensure!(amount > Zero::zero(), Error::<T>::InsufficientBalance);
            ensure!(required_approvals > 0, Error::<T>::InvalidMilestone);

            // Check milestone limit
            ensure!(
                escrow.milestones.len() < T::MaxMilestones::get() as usize,
                Error::<T>::TooManyMilestones
            );

            let bounded_description: BoundedVec<u8, ConstU32<256>> = description
                .try_into()
                .map_err(|_| Error::<T>::InvalidMilestone)?;

            let milestone = Milestone {
                id: escrow.next_milestone_id,
                description: bounded_description,
                amount,
                completed: false,
                approved_by: BoundedVec::new(),
                required_approvals,
            };

            escrow
                .milestones
                .try_push(milestone)
                .map_err(|_| Error::<T>::TooManyMilestones)?;
            escrow.is_milestone_based = true;

            let milestone_id = escrow.next_milestone_id;
            escrow.next_milestone_id = escrow.next_milestone_id.saturating_add(1);

            Escrows::<T>::insert(task_id, escrow);

            Self::deposit_event(Event::MilestoneAdded {
                task_id,
                milestone_id,
                amount,
                required_approvals,
            });

            Ok(())
        }

        /// Mark a milestone as completed
        #[pallet::call_index(8)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn complete_milestone(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            milestone_id: u32,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(
                escrow.state == EscrowState::Accepted,
                Error::<T>::InvalidEscrowState
            );

            // Only agent can mark milestone as completed
            ensure!(
                escrow.agent_account.as_ref() == Some(&caller),
                Error::<T>::NotAssignedAgent
            );

            // Find milestone
            let milestone = escrow
                .milestones
                .iter_mut()
                .find(|m| m.id == milestone_id)
                .ok_or(Error::<T>::MilestoneNotFound)?;

            ensure!(!milestone.completed, Error::<T>::MilestoneAlreadyCompleted);

            milestone.completed = true;
            Escrows::<T>::insert(task_id, escrow);

            Self::deposit_event(Event::MilestoneCompleted {
                task_id,
                milestone_id,
                completed_by: caller,
            });

            Ok(())
        }

        /// Approve a completed milestone
        #[pallet::call_index(9)]
        #[pallet::weight(Weight::from_parts(15_000, 0))]
        pub fn approve_milestone(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            milestone_id: u32,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(
                escrow.state == EscrowState::Accepted,
                Error::<T>::InvalidEscrowState
            );

            // Only user or participants can approve
            let is_authorized =
                escrow.user == caller || escrow.participants.iter().any(|p| p.account == caller);
            ensure!(is_authorized, Error::<T>::NotAuthorizedToApprove);

            // Find milestone
            let milestone = escrow
                .milestones
                .iter_mut()
                .find(|m| m.id == milestone_id)
                .ok_or(Error::<T>::MilestoneNotFound)?;

            ensure!(milestone.completed, Error::<T>::MilestoneNotCompleted);

            // Check if already approved by this account
            ensure!(
                !milestone.approved_by.contains(&caller),
                Error::<T>::AlreadyApproved
            );

            milestone
                .approved_by
                .try_push(caller.clone())
                .map_err(|_| Error::<T>::TooManyUserEscrows)?;

            // Check if milestone has enough approvals for payment
            let approval_count = milestone.approved_by.len() as u32;
            let should_pay = approval_count >= milestone.required_approvals;

            Escrows::<T>::insert(task_id, escrow.clone());

            Self::deposit_event(Event::MilestoneApproved {
                task_id,
                milestone_id,
                approved_by: caller.clone(),
            });

            // Auto-release payment if enough approvals
            if should_pay {
                Self::release_milestone_payment(&escrow, milestone_id)?;
            }

            Ok(())
        }

        // ========== PHASE 3: BATCH OPERATIONS ==========

        /// Create multiple escrows in a single atomic transaction
        #[pallet::call_index(10)]
        #[pallet::weight(Weight::from_parts(50_000u64.saturating_mul(requests.len() as u64), 0))]
        pub fn batch_create_escrow(
            origin: OriginFor<T>,
            requests: Vec<phase3_batch_refund::BatchCreateEscrowRequest<T>>,
        ) -> DispatchResult {
            let user = ensure_signed(origin)?;

            // Validate batch size
            ensure!(
                requests.len() <= T::MaxBatchSize::get() as usize,
                Error::<T>::BatchSizeExceeded
            );
            ensure!(!requests.is_empty(), Error::<T>::InvalidBatchSize);

            // Generate batch ID
            let batch_id = Self::generate_batch_id(&user, b"create_escrow");

            // Check if batch is already in progress
            ensure!(
                !BatchOperationsInProgress::<T>::contains_key(batch_id),
                Error::<T>::BatchAlreadyInProgress
            );

            // Pre-validate all requests and calculate total amount
            let mut total_amount = BalanceOf::<T>::zero();
            let mut validated_requests = Vec::new();

            for request in &requests {
                // Basic validations
                ensure!(
                    request.amount > Zero::zero(),
                    Error::<T>::InsufficientBalance
                );
                ensure!(
                    request.amount <= T::MaxEscrowAmount::get(),
                    Error::<T>::AmountTooLarge
                );
                ensure!(
                    !Escrows::<T>::contains_key(request.task_id),
                    Error::<T>::EscrowAlreadyExists
                );

                // Validate refund policy if present
                if let Some(ref policy) = request.refund_policy {
                    Self::validate_refund_policy(policy)?;
                }

                total_amount = total_amount
                    .checked_add(&request.amount)
                    .ok_or(Error::<T>::ArithmeticOverflow)?;

                validated_requests.push(request.clone());
            }

            // Check if user has sufficient balance for all operations
            let free_balance = T::Currency::free_balance(&user);
            ensure!(
                free_balance >= total_amount,
                Error::<T>::InsufficientBalanceForBatch
            );

            // Mark batch as in progress
            let current_block = <frame_system::Pallet<T>>::block_number();
            BatchOperationsInProgress::<T>::insert(batch_id, current_block);

            // Execute all operations atomically
            let mut successful_operations = 0u32;
            let mut first_failure_index = None;

            for (index, request) in validated_requests.iter().enumerate() {
                // Reserve funds first
                match T::Currency::reserve(&user, request.amount) {
                    Ok(_) => {
                        let timeout = request
                            .timeout_blocks
                            .unwrap_or_else(T::DefaultTimeout::get);
                        let expires_at = current_block + timeout;

                        let escrow = EscrowDetails {
                            task_id: request.task_id,
                            user: user.clone(),
                            agent_did: None,
                            agent_account: None,
                            amount: request.amount,
                            fee_percent: 5,
                            created_at: current_block,
                            expires_at,
                            state: EscrowState::Pending,
                            task_hash: request.task_hash,
                            participants: BoundedVec::new(),
                            is_multi_party: false,
                            milestones: BoundedVec::new(),
                            is_milestone_based: false,
                            next_milestone_id: 0,
                        };

                        // Insert escrow
                        Escrows::<T>::insert(request.task_id, escrow);

                        // Update user escrows
                        UserEscrows::<T>::try_mutate(&user, |tasks| {
                            tasks
                                .try_push(request.task_id)
                                .map_err(|_| Error::<T>::TooManyUserEscrows)
                        })?;

                        // Store refund policy if present
                        if let Some(ref policy) = request.refund_policy {
                            EscrowRefundPolicies::<T>::insert(request.task_id, policy);
                        }

                        successful_operations += 1;

                        // Emit individual escrow created event
                        Self::deposit_event(Event::EscrowCreated {
                            task_id: request.task_id,
                            user: user.clone(),
                            amount: request.amount,
                        });
                    }
                    Err(_) => {
                        first_failure_index = Some(index as u32);
                        break;
                    }
                }
            }

            // Clean up batch operation tracking
            BatchOperationsInProgress::<T>::remove(batch_id);

            // Update counters
            Self::increment_batch_counters(requests.len() as u32);

            // Emit batch completion event
            let operation_type = b"create_escrow"
                .to_vec()
                .try_into()
                .map_err(|_| Error::<T>::BatchOperationFailed)?;

            if first_failure_index.is_some() {
                let error_msg = b"InsufficientBalance"
                    .to_vec()
                    .try_into()
                    .map_err(|_| Error::<T>::BatchOperationFailed)?;

                Self::deposit_event(Event::BatchOperationFailed {
                    batch_id,
                    operation_type,
                    failure_index: first_failure_index.unwrap(),
                    error_message: error_msg,
                });

                Err(Error::<T>::InsufficientBalance.into())
            } else {
                Self::deposit_event(Event::BatchOperationCompleted {
                    batch_id,
                    operation_type,
                    successful_operations,
                    failed_operations: 0,
                    total_amount_processed: total_amount,
                });

                Ok(())
            }
        }

        /// Release payment for multiple escrows in one transaction
        #[pallet::call_index(11)]
        #[pallet::weight(Weight::from_parts(30_000u64.saturating_mul(task_ids.len() as u64), 0))]
        pub fn batch_release_payment(
            origin: OriginFor<T>,
            task_ids: Vec<[u8; 32]>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            // Validate batch size
            ensure!(
                task_ids.len() <= T::MaxBatchSize::get() as usize,
                Error::<T>::BatchSizeExceeded
            );
            ensure!(!task_ids.is_empty(), Error::<T>::InvalidBatchSize);

            let batch_id = Self::generate_batch_id(&caller, b"release_payment");

            // Pre-validate all escrows
            let mut total_amount = BalanceOf::<T>::zero();
            for task_id in &task_ids {
                let escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

                ensure!(escrow.user == caller, Error::<T>::NotEscrowCreator);
                ensure!(
                    escrow.state == EscrowState::Accepted,
                    Error::<T>::InvalidEscrowState
                );

                total_amount = total_amount.saturating_add(escrow.amount);
            }

            // Execute batch release
            let mut successful_operations = 0u32;

            for task_id in &task_ids {
                let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

                let agent = escrow
                    .agent_account
                    .clone()
                    .ok_or(Error::<T>::InvalidEscrowState)?;

                let fee_amount = Self::calculate_fee(escrow.amount, escrow.fee_percent)?;
                let net_amount = escrow
                    .amount
                    .checked_sub(&fee_amount)
                    .ok_or(Error::<T>::ArithmeticOverflow)?;

                T::Currency::unreserve(&escrow.user, escrow.amount);

                T::Currency::transfer(
                    &escrow.user,
                    &agent,
                    net_amount,
                    ExistenceRequirement::KeepAlive,
                )?;

                T::Currency::transfer(
                    &escrow.user,
                    &T::ProtocolFeeAccount::get(),
                    fee_amount,
                    ExistenceRequirement::AllowDeath,
                )?;

                escrow.state = EscrowState::Completed;
                Escrows::<T>::insert(task_id, escrow);

                successful_operations += 1;

                Self::deposit_event(Event::PaymentReleased {
                    task_id: *task_id,
                    agent: agent.clone(),
                    amount: net_amount,
                    fee: fee_amount,
                });
            }

            let operation_type = b"release_payment"
                .to_vec()
                .try_into()
                .map_err(|_| Error::<T>::BatchOperationFailed)?;

            Self::deposit_event(Event::BatchOperationCompleted {
                batch_id,
                operation_type,
                successful_operations,
                failed_operations: 0,
                total_amount_processed: total_amount,
            });

            Ok(())
        }

        /// Refund multiple escrows with policy enforcement
        #[pallet::call_index(12)]
        #[pallet::weight(Weight::from_parts(35_000u64.saturating_mul(task_ids.len() as u64), 0))]
        pub fn batch_refund_escrow(
            origin: OriginFor<T>,
            task_ids: Vec<[u8; 32]>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            // Validate batch size
            ensure!(
                task_ids.len() <= T::MaxBatchSize::get() as usize,
                Error::<T>::BatchSizeExceeded
            );
            ensure!(!task_ids.is_empty(), Error::<T>::InvalidBatchSize);

            let batch_id = Self::generate_batch_id(&caller, b"refund_escrow");
            let mut total_amount = BalanceOf::<T>::zero();
            let mut successful_operations = 0u32;

            for task_id in &task_ids {
                let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

                ensure!(
                    escrow.state == EscrowState::Pending || escrow.state == EscrowState::Accepted,
                    Error::<T>::InvalidEscrowState
                );

                let current_block = <frame_system::Pallet<T>>::block_number();
                let is_expired = current_block >= escrow.expires_at;

                // Check authorization
                if escrow.state == EscrowState::Pending {
                    ensure!(escrow.user == caller, Error::<T>::NotEscrowCreator);
                } else if escrow.state == EscrowState::Accepted {
                    ensure!(is_expired, Error::<T>::EscrowNotExpired);
                }

                // Calculate refund amount based on policy
                let refund_amount = if let Some(policy) = EscrowRefundPolicies::<T>::get(task_id) {
                    Self::evaluate_refund_policy(task_id, &policy, escrow.amount)?
                } else {
                    escrow.amount // Standard policy - full refund
                };

                // Unreserve and refund
                T::Currency::unreserve(&escrow.user, escrow.amount);
                if refund_amount > Zero::zero() && refund_amount < escrow.amount {
                    // Partial refund - return the difference to protocol
                    let protocol_amount = escrow.amount.saturating_sub(refund_amount);
                    T::Currency::transfer(
                        &escrow.user,
                        &T::ProtocolFeeAccount::get(),
                        protocol_amount,
                        ExistenceRequirement::AllowDeath,
                    )?;
                }

                escrow.state = EscrowState::Refunded;
                Escrows::<T>::insert(task_id, escrow.clone());

                total_amount = total_amount.saturating_add(refund_amount);
                successful_operations += 1;

                Self::deposit_event(Event::EscrowRefunded {
                    task_id: *task_id,
                    user: escrow.user,
                    amount: refund_amount,
                });
            }

            let operation_type = b"refund_escrow"
                .to_vec()
                .try_into()
                .map_err(|_| Error::<T>::BatchOperationFailed)?;

            Self::deposit_event(Event::BatchOperationCompleted {
                batch_id,
                operation_type,
                successful_operations,
                failed_operations: 0,
                total_amount_processed: total_amount,
            });

            Ok(())
        }

        /// Dispute multiple escrows at once
        #[pallet::call_index(13)]
        #[pallet::weight(Weight::from_parts(25_000u64.saturating_mul(task_ids.len() as u64), 0))]
        pub fn batch_dispute_escrow(
            origin: OriginFor<T>,
            task_ids: Vec<[u8; 32]>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            // Validate batch size
            ensure!(
                task_ids.len() <= T::MaxBatchSize::get() as usize,
                Error::<T>::BatchSizeExceeded
            );
            ensure!(!task_ids.is_empty(), Error::<T>::InvalidBatchSize);

            let batch_id = Self::generate_batch_id(&caller, b"dispute_escrow");
            let mut successful_operations = 0u32;

            for task_id in &task_ids {
                let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

                ensure!(
                    escrow.state == EscrowState::Accepted,
                    Error::<T>::InvalidEscrowState
                );

                let is_user = escrow.user == caller;
                let is_agent = escrow.agent_account.as_ref() == Some(&caller);
                ensure!(is_user || is_agent, Error::<T>::NotEscrowCreator);

                escrow.state = EscrowState::Disputed;
                Escrows::<T>::insert(task_id, escrow);

                successful_operations += 1;

                Self::deposit_event(Event::DisputeRaised {
                    task_id: *task_id,
                    raised_by: caller.clone(),
                });
            }

            let operation_type = b"dispute_escrow"
                .to_vec()
                .try_into()
                .map_err(|_| Error::<T>::BatchOperationFailed)?;

            Self::deposit_event(Event::BatchOperationCompleted {
                batch_id,
                operation_type,
                successful_operations,
                failed_operations: 0,
                total_amount_processed: Zero::zero(),
            });

            Ok(())
        }

        // ========== PHASE 3: REFUND POLICY MANAGEMENT ==========

        /// Set a refund policy for an escrow
        #[pallet::call_index(14)]
        #[pallet::weight(Weight::from_parts(20_000, 0))]
        pub fn set_refund_policy(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            policy: phase3_batch_refund::RefundPolicy<T>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            ensure!(escrow.user == caller, Error::<T>::NotEscrowCreator);
            ensure!(
                escrow.state == EscrowState::Pending,
                Error::<T>::InvalidEscrowState
            );

            // Validate the policy
            Self::validate_refund_policy(&policy)?;

            // Store the policy
            EscrowRefundPolicies::<T>::insert(task_id, &policy);

            // Emit event
            let policy_type = Self::get_policy_type_name(&policy.policy_type);
            Self::deposit_event(Event::RefundPolicySet {
                task_id,
                policy_type,
                can_override: policy.can_override,
            });

            Ok(())
        }

        /// Update a refund policy (if allowed)
        #[pallet::call_index(15)]
        #[pallet::weight(Weight::from_parts(20_000, 0))]
        pub fn update_refund_policy(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            new_policy: phase3_batch_refund::RefundPolicy<T>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            let old_policy =
                EscrowRefundPolicies::<T>::get(task_id).ok_or(Error::<T>::RefundPolicyNotFound)?;

            // Check authorization
            ensure!(
                escrow.user == caller || Self::can_override_policy(&old_policy, &caller),
                Error::<T>::NotAuthorizedToOverride
            );

            // Validate new policy
            Self::validate_refund_policy(&new_policy)?;

            // Update policy
            EscrowRefundPolicies::<T>::insert(task_id, &new_policy);

            let old_policy_name = Self::get_policy_type_name(&old_policy.policy_type);
            let new_policy_name = Self::get_policy_type_name(&new_policy.policy_type);

            Self::deposit_event(Event::RefundPolicyUpdated {
                task_id,
                old_policy: old_policy_name,
                new_policy: new_policy_name,
                updated_by: caller,
            });

            Ok(())
        }

        /// Evaluate refund amount based on current policy
        #[pallet::call_index(16)]
        #[pallet::weight(Weight::from_parts(15_000, 0))]
        pub fn evaluate_refund_amount(origin: OriginFor<T>, task_id: [u8; 32]) -> DispatchResult {
            let _caller = ensure_signed(origin)?;

            let escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            let refund_amount = if let Some(policy) = EscrowRefundPolicies::<T>::get(task_id) {
                Self::evaluate_refund_policy(&task_id, &policy, escrow.amount)?
            } else {
                escrow.amount // Standard policy
            };

            let policy_type = if let Some(policy) = EscrowRefundPolicies::<T>::get(task_id) {
                Self::get_policy_type_name(&policy.policy_type)
            } else {
                b"Standard"
                    .to_vec()
                    .try_into()
                    .map_err(|_| Error::<T>::InvalidRefundPolicy)?
            };

            Self::deposit_event(Event::RefundAmountCalculated {
                task_id,
                policy_type,
                original_amount: escrow.amount,
                refund_amount,
            });

            Ok(())
        }

        /// Override refund policy (admin/arbitrator only)
        #[pallet::call_index(17)]
        #[pallet::weight(Weight::from_parts(25_000, 0))]
        pub fn override_refund_amount(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            override_amount: BalanceOf<T>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

            let policy =
                EscrowRefundPolicies::<T>::get(task_id).ok_or(Error::<T>::RefundPolicyNotFound)?;

            ensure!(
                Self::can_override_policy(&policy, &caller),
                Error::<T>::CannotOverridePolicy
            );

            ensure!(
                override_amount <= escrow.amount,
                Error::<T>::InvalidRefundPercentage
            );

            // Execute the override refund
            let refund_amount = override_amount;
            T::Currency::unreserve(&escrow.user, escrow.amount);

            if refund_amount < escrow.amount {
                let protocol_amount = escrow.amount.saturating_sub(refund_amount);
                T::Currency::transfer(
                    &escrow.user,
                    &T::ProtocolFeeAccount::get(),
                    protocol_amount,
                    ExistenceRequirement::AllowDeath,
                )?;
            }

            escrow.state = EscrowState::Refunded;
            Escrows::<T>::insert(task_id, escrow.clone());

            Self::deposit_event(Event::RefundPolicyOverridden {
                task_id,
                original_amount: escrow.amount,
                override_amount: refund_amount,
                overridden_by: caller,
            });

            Self::deposit_event(Event::EscrowRefunded {
                task_id,
                user: escrow.user,
                amount: refund_amount,
            });

            Ok(())
        }

        // ========== PHASE 2: TEMPLATE SYSTEM ==========

        /// Create a custom escrow template
        #[pallet::call_index(18)]
        #[pallet::weight(Weight::from_parts(30_000, 0))]
        pub fn create_template(
            origin: OriginFor<T>,
            name: Vec<u8>,
            description: Vec<u8>,
            template_type: templates::TemplateType,
            params: templates::TemplateParams<T>,
        ) -> DispatchResult {
            let creator = ensure_signed(origin)?;

            // Validate template parameters
            Self::validate_template_params(&params)?;

            // Get next template ID
            let template_id = NextTemplateId::<T>::get();

            // Create template
            let template = if template_type == templates::TemplateType::Custom {
                templates::EscrowTemplate::custom(
                    template_id,
                    name,
                    description,
                    params,
                    creator.clone(),
                    <frame_system::Pallet<T>>::block_number(),
                )?
            } else {
                // For non-custom templates, use the predefined ones
                return Err(Error::<T>::InvalidTemplateParams.into());
            };

            // Store template
            EscrowTemplates::<T>::insert(template_id, &template);

            // Update template ID counter
            NextTemplateId::<T>::set(template_id + 1);

            // Index by creator
            TemplatesByCreator::<T>::try_mutate(&creator, |template_ids| {
                template_ids
                    .try_push(template_id)
                    .map_err(|_| Error::<T>::TooManyTemplates)
            })?;

            let template_type_name = match template.template_type {
                templates::TemplateType::Custom => b"Custom".to_vec(),
                templates::TemplateType::SimplePayment => b"SimplePayment".to_vec(),
                templates::TemplateType::MilestoneProject => b"MilestoneProject".to_vec(),
                templates::TemplateType::MultiPartyContract => b"MultiPartyContract".to_vec(),
                templates::TemplateType::TimeLockedRelease => b"TimeLockedRelease".to_vec(),
                templates::TemplateType::ConditionalPayment => b"ConditionalPayment".to_vec(),
                templates::TemplateType::EscrowedPurchase => b"EscrowedPurchase".to_vec(),
                templates::TemplateType::SubscriptionPayment => b"SubscriptionPayment".to_vec(),
            }
            .try_into()
            .map_err(|_| Error::<T>::InvalidTemplateParams)?;

            Self::deposit_event(Event::TemplateCreated {
                template_id,
                template_type: template_type_name,
                created_by: creator,
            });

            Ok(())
        }

        /// Create an escrow from a template
        #[pallet::call_index(19)]
        #[pallet::weight(Weight::from_parts(40_000, 0))]
        pub fn create_escrow_from_template(
            origin: OriginFor<T>,
            task_id: [u8; 32],
            amount: BalanceOf<T>,
            task_hash: [u8; 32],
            config: templates::TemplateEscrowConfig<T>,
        ) -> DispatchResult {
            let user = ensure_signed(origin)?;

            // Validate basic parameters
            ensure!(amount > Zero::zero(), Error::<T>::InsufficientBalance);
            ensure!(
                amount <= T::MaxEscrowAmount::get(),
                Error::<T>::AmountTooLarge
            );
            ensure!(
                !Escrows::<T>::contains_key(task_id),
                Error::<T>::EscrowAlreadyExists
            );

            // Get template
            let template = EscrowTemplates::<T>::get(config.template_id)
                .ok_or(Error::<T>::TemplateNotFound)?;

            ensure!(template.is_active, Error::<T>::TemplateInactive);

            // Validate amount against template limits
            if let Some(min_amount) = template.default_params.min_amount {
                ensure!(amount >= min_amount, Error::<T>::InsufficientBalance);
            }
            if let Some(max_amount) = template.default_params.max_amount {
                ensure!(amount <= max_amount, Error::<T>::AmountTooLarge);
            }

            // Reserve funds
            T::Currency::reserve(&user, amount).map_err(|_| Error::<T>::InsufficientBalance)?;

            let current_block = <frame_system::Pallet<T>>::block_number();
            let default_timeout = config
                .timeout_override
                .or(template.default_params.default_timeout)
                .unwrap_or_else(T::DefaultTimeout::get);
            let expires_at = current_block + default_timeout;

            let fee_percent = config
                .fee_percent_override
                .or(template.default_params.default_fee_percent)
                .unwrap_or(5u8);

            // Create base escrow
            let mut escrow = EscrowDetails {
                task_id,
                user: user.clone(),
                agent_did: None,
                agent_account: None,
                amount,
                fee_percent,
                created_at: current_block,
                expires_at,
                state: EscrowState::Pending,
                task_hash,
                participants: BoundedVec::new(),
                is_multi_party: false,
                milestones: BoundedVec::new(),
                is_milestone_based: false,
                next_milestone_id: 0,
            };

            // Apply template configuration
            Self::apply_template_config(&template, &config, &mut escrow)?;

            // Store escrow
            Escrows::<T>::insert(task_id, escrow);

            // Update user escrows
            UserEscrows::<T>::try_mutate(&user, |tasks| {
                tasks
                    .try_push(task_id)
                    .map_err(|_| Error::<T>::TooManyUserEscrows)
            })?;

            // Increment template usage
            Self::increment_template_usage(config.template_id)?;

            Self::deposit_event(Event::EscrowCreatedFromTemplate {
                task_id,
                template_id: config.template_id,
                user,
                amount,
            });

            Ok(())
        }

        /// Update a template (only creator can update custom templates)
        #[pallet::call_index(20)]
        #[pallet::weight(Weight::from_parts(25_000, 0))]
        pub fn update_template(
            origin: OriginFor<T>,
            template_id: u32,
            name: Option<Vec<u8>>,
            description: Option<Vec<u8>>,
            params: Option<templates::TemplateParams<T>>,
        ) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut template =
                EscrowTemplates::<T>::get(template_id).ok_or(Error::<T>::TemplateNotFound)?;

            // Only creator can update, and only custom templates
            ensure!(
                template.created_by == caller,
                Error::<T>::NotTemplateCreator
            );
            ensure!(
                template.template_type == templates::TemplateType::Custom,
                Error::<T>::CannotUpdateBuiltinTemplate
            );

            // Update fields if provided
            if let Some(new_name) = name {
                let bounded_name = new_name
                    .try_into()
                    .map_err(|_| Error::<T>::TemplateNameTooLong)?;
                template.name = bounded_name;
            }

            if let Some(new_description) = description {
                let bounded_description = new_description
                    .try_into()
                    .map_err(|_| Error::<T>::TemplateDescriptionTooLong)?;
                template.description = bounded_description;
            }

            if let Some(new_params) = params {
                Self::validate_template_params(&new_params)?;
                template.default_params = new_params;
            }

            // Store updated template
            EscrowTemplates::<T>::insert(template_id, &template);

            Self::deposit_event(Event::TemplateUpdated {
                template_id,
                updated_by: caller,
            });

            Ok(())
        }

        /// Deactivate a template (only creator can deactivate custom templates)
        #[pallet::call_index(21)]
        #[pallet::weight(Weight::from_parts(15_000, 0))]
        pub fn deactivate_template(origin: OriginFor<T>, template_id: u32) -> DispatchResult {
            let caller = ensure_signed(origin)?;

            let mut template =
                EscrowTemplates::<T>::get(template_id).ok_or(Error::<T>::TemplateNotFound)?;

            // Only creator can deactivate custom templates
            if template.template_type == templates::TemplateType::Custom {
                ensure!(
                    template.created_by == caller,
                    Error::<T>::NotTemplateCreator
                );
            }

            template.is_active = false;
            EscrowTemplates::<T>::insert(template_id, &template);

            Self::deposit_event(Event::TemplateDeactivated {
                template_id,
                deactivated_by: caller,
            });

            Ok(())
        }
    }

    impl<T: Config> Pallet<T> {
        fn calculate_fee(amount: BalanceOf<T>, fee_percent: u8) -> Result<BalanceOf<T>, Error<T>> {
            let fee_multiplier = BalanceOf::<T>::from(fee_percent as u32);
            let hundred = BalanceOf::<T>::from(100u32);

            amount
                .checked_mul(&fee_multiplier)
                .and_then(|v| v.checked_div(&hundred))
                .ok_or(Error::<T>::ArithmeticOverflow)
        }

        pub fn is_expired(task_id: &[u8; 32]) -> bool {
            if let Some(escrow) = Escrows::<T>::get(task_id) {
                let current_block = <frame_system::Pallet<T>>::block_number();
                current_block >= escrow.expires_at
            } else {
                false
            }
        }

        pub fn get_escrow(task_id: &[u8; 32]) -> Option<EscrowDetails<T>> {
            Escrows::<T>::get(task_id)
        }

        /// Release payment for a milestone
        pub fn release_milestone_payment(
            escrow: &EscrowDetails<T>,
            milestone_id: u32,
        ) -> DispatchResult {
            // Find milestone
            let milestone = escrow
                .milestones
                .iter()
                .find(|m| m.id == milestone_id)
                .ok_or(Error::<T>::MilestoneNotFound)?;

            let agent = escrow
                .agent_account
                .as_ref()
                .ok_or(Error::<T>::InvalidEscrowState)?;

            let fee_amount = Self::calculate_fee(milestone.amount, escrow.fee_percent)?;
            let net_amount = milestone
                .amount
                .checked_sub(&fee_amount)
                .ok_or(Error::<T>::ArithmeticOverflow)?;

            // Transfer from escrow creator to agent
            T::Currency::transfer(
                &escrow.user,
                agent,
                net_amount,
                ExistenceRequirement::KeepAlive,
            )?;

            // Transfer fee to protocol account
            T::Currency::transfer(
                &escrow.user,
                &T::ProtocolFeeAccount::get(),
                fee_amount,
                ExistenceRequirement::AllowDeath,
            )?;

            Self::deposit_event(Event::MilestonePaid {
                task_id: escrow.task_id,
                milestone_id,
                amount: net_amount,
                recipient: agent.clone(),
            });

            Ok(())
        }

        /// Release payment for multi-party escrow
        pub fn release_multi_party_payment(escrow: &EscrowDetails<T>) -> DispatchResult {
            // Check that all participants are approved
            let all_approved = escrow.participants.iter().all(|p| p.approved);
            ensure!(all_approved, Error::<T>::InsufficientApprovals);

            let mut total_amount: BalanceOf<T> = Zero::zero();

            // Process payments to all payees
            for participant in &escrow.participants {
                if participant.role == ParticipantRole::Payee {
                    let fee_amount = Self::calculate_fee(participant.amount, escrow.fee_percent)?;
                    let net_amount = participant
                        .amount
                        .checked_sub(&fee_amount)
                        .ok_or(Error::<T>::ArithmeticOverflow)?;

                    // Find corresponding payer(s) and transfer
                    for payer in &escrow.participants {
                        if payer.role == ParticipantRole::Payer {
                            // Transfer from payer to payee
                            T::Currency::unreserve(&payer.account, payer.amount);
                            T::Currency::transfer(
                                &payer.account,
                                &participant.account,
                                net_amount,
                                ExistenceRequirement::KeepAlive,
                            )?;

                            // Transfer fee to protocol account
                            T::Currency::transfer(
                                &payer.account,
                                &T::ProtocolFeeAccount::get(),
                                fee_amount,
                                ExistenceRequirement::AllowDeath,
                            )?;

                            total_amount = total_amount.saturating_add(participant.amount);
                            break;
                        }
                    }
                }
            }

            Self::deposit_event(Event::MultiPartyRelease {
                task_id: escrow.task_id,
                total_amount,
                participants_count: escrow.participants.len() as u32,
            });

            Ok(())
        }

        /// Check if participant is authorized for an operation
        pub fn is_participant(escrow: &EscrowDetails<T>, account: &T::AccountId) -> bool {
            escrow.participants.iter().any(|p| &p.account == account)
        }

        /// Get milestone by ID
        pub fn get_milestone(
            escrow: &EscrowDetails<T>,
            milestone_id: u32,
        ) -> Option<&Milestone<T>> {
            escrow.milestones.iter().find(|m| m.id == milestone_id)
        }

        // ========== PHASE 3: HELPER FUNCTIONS ==========

        /// Validates a refund policy
        pub fn validate_refund_policy(
            policy: &phase3_batch_refund::RefundPolicy<T>,
        ) -> DispatchResult {
            match &policy.policy_type {
                phase3_batch_refund::RefundPolicyType::TimeBased {
                    partial_refund_percentage,
                    ..
                } => {
                    ensure!(
                        *partial_refund_percentage <= 100u8,
                        Error::<T>::InvalidRefundPercentage
                    );
                }
                phase3_batch_refund::RefundPolicyType::Graduated { stages } => {
                    ensure!(!stages.is_empty(), Error::<T>::GraduatedStagesInvalid);

                    // Validate stages are in ascending order and percentages valid
                    let mut last_block = BlockNumberFor::<T>::zero();
                    for (block, percentage) in stages.iter() {
                        ensure!(*block > last_block, Error::<T>::GraduatedStagesInvalid);
                        ensure!(*percentage <= 100u8, Error::<T>::InvalidRefundPercentage);
                        last_block = *block;
                    }
                }
                phase3_batch_refund::RefundPolicyType::Conditional {
                    refund_percentages, ..
                } => {
                    ensure!(
                        !refund_percentages.is_empty(),
                        Error::<T>::ConditionalMilestonesInvalid
                    );

                    // Validate all percentages
                    for percentage in refund_percentages.iter() {
                        ensure!(*percentage <= 100u8, Error::<T>::InvalidRefundPercentage);
                    }
                }
                phase3_batch_refund::RefundPolicyType::CancellationFee { fee_amount } => {
                    ensure!(*fee_amount > Zero::zero(), Error::<T>::InvalidRefundPolicy);
                }
                phase3_batch_refund::RefundPolicyType::NoRefund { .. }
                | phase3_batch_refund::RefundPolicyType::DisputeBased
                | phase3_batch_refund::RefundPolicyType::Standard => {
                    // These are always valid
                }
            }

            Ok(())
        }

        /// Evaluates refund policy and calculates refund amount
        pub fn evaluate_refund_policy(
            task_id: &[u8; 32],
            policy: &phase3_batch_refund::RefundPolicy<T>,
            original_amount: BalanceOf<T>,
        ) -> Result<BalanceOf<T>, DispatchError> {
            let current_block = <frame_system::Pallet<T>>::block_number();

            match &policy.policy_type {
                phase3_batch_refund::RefundPolicyType::Standard => {
                    // Standard policy - full refund
                    Ok(original_amount)
                }

                phase3_batch_refund::RefundPolicyType::TimeBased {
                    full_refund_deadline,
                    partial_refund_percentage,
                } => {
                    if current_block <= *full_refund_deadline {
                        Ok(original_amount) // Full refund
                    } else {
                        // Partial refund
                        let percentage = BalanceOf::<T>::from(*partial_refund_percentage as u32);
                        let hundred = BalanceOf::<T>::from(100u32);
                        original_amount
                            .checked_mul(&percentage)
                            .and_then(|v| v.checked_div(&hundred))
                            .ok_or(Error::<T>::ArithmeticOverflow.into())
                    }
                }

                phase3_batch_refund::RefundPolicyType::Graduated { stages } => {
                    // Find appropriate stage based on current block
                    let mut refund_percentage = 100u8;

                    for (deadline, percentage) in stages.iter() {
                        if current_block <= *deadline {
                            break;
                        }
                        refund_percentage = *percentage;
                    }

                    let percentage = BalanceOf::<T>::from(refund_percentage as u32);
                    let hundred = BalanceOf::<T>::from(100u32);
                    original_amount
                        .checked_mul(&percentage)
                        .and_then(|v| v.checked_div(&hundred))
                        .ok_or(Error::<T>::ArithmeticOverflow.into())
                }

                phase3_batch_refund::RefundPolicyType::CancellationFee { fee_amount } => {
                    Ok(original_amount
                        .checked_sub(fee_amount)
                        .unwrap_or(Zero::zero()))
                }

                phase3_batch_refund::RefundPolicyType::NoRefund {
                    work_start_deadline,
                } => {
                    if current_block <= *work_start_deadline {
                        Ok(original_amount) // Full refund before work starts
                    } else {
                        Ok(Zero::zero()) // No refund after work starts
                    }
                }

                phase3_batch_refund::RefundPolicyType::Conditional {
                    milestones_completed,
                    refund_percentages,
                } => {
                    let escrow = Escrows::<T>::get(task_id).ok_or(Error::<T>::EscrowNotFound)?;

                    // Count completed milestones
                    let completed_count =
                        escrow.milestones.iter().filter(|m| m.completed).count() as u8;

                    // Find appropriate refund percentage
                    let percentage = if (*milestones_completed as usize) < refund_percentages.len()
                    {
                        refund_percentages[completed_count.min(*milestones_completed) as usize]
                    } else {
                        0u8 // No refund if beyond defined milestones
                    };

                    let percentage_balance = BalanceOf::<T>::from(percentage as u32);
                    let hundred = BalanceOf::<T>::from(100u32);
                    original_amount
                        .checked_mul(&percentage_balance)
                        .and_then(|v| v.checked_div(&hundred))
                        .ok_or(Error::<T>::ArithmeticOverflow.into())
                }

                phase3_batch_refund::RefundPolicyType::DisputeBased => {
                    // Dispute-based policies require manual arbitration
                    // Return original amount as placeholder
                    Ok(original_amount)
                }
            }
        }

        /// Checks if a refund policy can be overridden
        pub fn can_override_policy(
            policy: &phase3_batch_refund::RefundPolicy<T>,
            caller: &T::AccountId,
        ) -> bool {
            if !policy.can_override {
                return false;
            }

            if let Some(ref authority) = policy.override_authority {
                authority == caller
            } else {
                true // No specific authority required
            }
        }

        /// Generates a unique batch ID
        pub fn generate_batch_id(user: &T::AccountId, operation_type: &[u8]) -> [u8; 32] {
            let current_block = <frame_system::Pallet<T>>::block_number();
            let (counters, _) = Self::batch_operation_counters();

            let mut data = Vec::new();
            data.extend_from_slice(&user.encode());
            data.extend_from_slice(operation_type);
            data.extend_from_slice(&current_block.encode());
            data.extend_from_slice(&counters.encode());

            let mut batch_id = [0u8; 32];
            let hash = frame_support::Hashable::blake2_256(&data);
            batch_id.copy_from_slice(&hash);
            batch_id
        }

        /// Updates batch operation counters
        pub fn increment_batch_counters(operations_count: u32) {
            BatchOperationCounters::<T>::mutate(|(total_batches, total_operations)| {
                *total_batches = total_batches.saturating_add(1);
                *total_operations = total_operations.saturating_add(operations_count as u64);
            });
        }

        /// Gets policy type name for events
        pub fn get_policy_type_name(
            policy_type: &phase3_batch_refund::RefundPolicyType<T>,
        ) -> BoundedVec<u8, ConstU32<32>> {
            use phase3_batch_refund::RefundPolicyType;

            let name = match policy_type {
                RefundPolicyType::TimeBased { .. } => "TimeBased",
                RefundPolicyType::Graduated { .. } => "Graduated",
                RefundPolicyType::CancellationFee { .. } => "CancellationFee",
                RefundPolicyType::NoRefund { .. } => "NoRefund",
                RefundPolicyType::Conditional { .. } => "Conditional",
                RefundPolicyType::DisputeBased => "DisputeBased",
                RefundPolicyType::Standard => "Standard",
            };

            name.as_bytes().to_vec().try_into().unwrap_or_else(|_| {
                b"Unknown"
                    .to_vec()
                    .try_into()
                    .expect("Unknown should fit in BoundedVec")
            })
        }

        // ========== PHASE 2: TEMPLATE HELPER FUNCTIONS ==========

        /// Validates template parameters for consistency and safety
        pub fn validate_template_params(params: &templates::TemplateParams<T>) -> DispatchResult {
            // Validate fee percentage
            if let Some(fee_percent) = params.default_fee_percent {
                ensure!(fee_percent <= 100u8, Error::<T>::InvalidFeePercentage);
            }

            // Validate participant limits
            if let Some(max_participants) = params.max_participants {
                ensure!(
                    max_participants > 0 && max_participants <= 1000,
                    Error::<T>::InvalidTemplateParams
                );
            }

            // Validate milestone limits
            if let Some(max_milestones) = params.max_milestones {
                ensure!(
                    max_milestones > 0 && max_milestones <= 100,
                    Error::<T>::InvalidTemplateParams
                );
            }

            // Validate milestone approvals don't exceed milestones
            if let (Some(max_milestones), Some(milestone_approvals)) =
                (params.max_milestones, params.default_milestone_approvals)
            {
                ensure!(
                    milestone_approvals <= max_milestones,
                    Error::<T>::InvalidTemplateParams
                );
            }

            // Validate amount range
            if let (Some(min_amount), Some(max_amount)) = (params.min_amount, params.max_amount) {
                ensure!(min_amount <= max_amount, Error::<T>::InvalidAmountRange);
            }

            // Validate timeout values
            if let Some(timeout) = params.default_timeout {
                let current_block = <frame_system::Pallet<T>>::block_number();
                ensure!(timeout > current_block, Error::<T>::InvalidTemplateParams);
            }

            // Auto timeouts should be less than default timeout
            if let (Some(default_timeout), Some(auto_accept)) =
                (params.default_timeout, params.auto_accept_timeout)
            {
                ensure!(
                    auto_accept < default_timeout,
                    Error::<T>::InvalidTemplateParams
                );
            }

            if let (Some(default_timeout), Some(auto_release)) =
                (params.default_timeout, params.auto_release_timeout)
            {
                ensure!(
                    auto_release < default_timeout,
                    Error::<T>::InvalidTemplateParams
                );
            }

            Ok(())
        }

        /// Increments the usage count for a template
        pub fn increment_template_usage(template_id: u32) -> DispatchResult {
            EscrowTemplates::<T>::try_mutate(template_id, |template_opt| {
                let template = template_opt.as_mut().ok_or(Error::<T>::TemplateNotFound)?;

                ensure!(template.is_active, Error::<T>::TemplateInactive);

                template.usage_count = template.usage_count.saturating_add(1);

                Ok(())
            })
        }

        /// Applies template configuration to an escrow, with overrides support
        pub fn apply_template_config(
            template: &templates::EscrowTemplate<T>,
            config: &templates::TemplateEscrowConfig<T>,
            escrow: &mut EscrowDetails<T>,
        ) -> DispatchResult {
            // Apply default timeout or override
            let timeout = config.timeout_override.unwrap_or(
                template.default_params.default_timeout.unwrap_or_else(|| {
                    let current_block = <frame_system::Pallet<T>>::block_number();
                    current_block + T::DefaultTimeout::get()
                }),
            );
            escrow.expires_at = timeout;

            // Apply fee percentage or override
            let fee_percent = config
                .fee_percent_override
                .unwrap_or(template.default_params.default_fee_percent.unwrap_or(5u8));
            ensure!(fee_percent <= 100u8, Error::<T>::InvalidFeePercentage);
            escrow.fee_percent = fee_percent;

            // Validate amount is within template bounds
            if let Some(min_amount) = template.default_params.min_amount {
                ensure!(escrow.amount >= min_amount, Error::<T>::InsufficientBalance);
            }
            if let Some(max_amount) = template.default_params.max_amount {
                ensure!(escrow.amount <= max_amount, Error::<T>::AmountTooLarge);
            }

            // Apply multi-party configuration if enabled
            if template.default_params.multi_party_enabled {
                if let Some(participant_configs) = &config.participant_configs {
                    ensure!(
                        !participant_configs.is_empty(),
                        Error::<T>::InvalidTemplateParams
                    );

                    if let Some(max_participants) = template.default_params.max_participants {
                        ensure!(
                            participant_configs.len() <= max_participants as usize,
                            Error::<T>::TooManyParticipants
                        );
                    }

                    // Convert participant configs to escrow participants
                    for (account, role, amount) in participant_configs {
                        let participant = EscrowParticipant {
                            account: account.clone(),
                            role: role.clone(),
                            amount: *amount,
                            approved: false,
                        };
                        escrow
                            .participants
                            .try_push(participant)
                            .map_err(|_| Error::<T>::TooManyParticipants)?;
                    }
                    escrow.is_multi_party = true;
                }
            }

            // Apply milestone configuration if enabled
            if template.default_params.milestone_enabled {
                if let Some(milestone_configs) = &config.milestone_configs {
                    ensure!(
                        !milestone_configs.is_empty(),
                        Error::<T>::InvalidTemplateParams
                    );

                    if let Some(max_milestones) = template.default_params.max_milestones {
                        ensure!(
                            milestone_configs.len() <= max_milestones.saturated_into::<usize>(),
                            Error::<T>::TooManyMilestones
                        );
                    }

                    // Convert milestone configs to escrow milestones
                    for (description, amount, required_approvals) in milestone_configs {
                        let bounded_description: BoundedVec<u8, ConstU32<256>> = description
                            .clone()
                            .try_into()
                            .map_err(|_| Error::<T>::InvalidTemplateParams)?;

                        let milestone = Milestone {
                            id: escrow.next_milestone_id,
                            description: bounded_description,
                            amount: *amount,
                            completed: false,
                            approved_by: BoundedVec::new(),
                            required_approvals: *required_approvals,
                        };
                        escrow
                            .milestones
                            .try_push(milestone)
                            .map_err(|_| Error::<T>::TooManyMilestones)?;
                        escrow.next_milestone_id = escrow.next_milestone_id.saturating_add(1);
                    }
                    escrow.is_milestone_based = true;
                }
            }

            // Apply auto-accept timeout if configured
            if let Some(auto_accept_timeout) = template.default_params.auto_accept_timeout {
                ensure!(
                    auto_accept_timeout < escrow.expires_at,
                    Error::<T>::InvalidTemplateParams
                );
                // Store in metadata or extend escrow struct if needed
            }

            // Note: disputes_enabled configuration is handled through template validation
            // and can be checked via the template when needed

            Ok(())
        }

        /// Gets template type name for events
        pub fn get_template_type_name(
            template_type: &templates::TemplateType,
        ) -> BoundedVec<u8, ConstU32<32>> {
            let name = match template_type {
                templates::TemplateType::SimplePayment => "SimplePayment",
                templates::TemplateType::MilestoneProject => "MilestoneProject",
                templates::TemplateType::MultiPartyContract => "MultiPartyContract",
                templates::TemplateType::TimeLockedRelease => "TimeLockedRelease",
                templates::TemplateType::ConditionalPayment => "ConditionalPayment",
                templates::TemplateType::EscrowedPurchase => "EscrowedPurchase",
                templates::TemplateType::SubscriptionPayment => "SubscriptionPayment",
                templates::TemplateType::Custom => "Custom",
            };

            name.as_bytes().to_vec().try_into().unwrap_or_else(|_| {
                b"Unknown"
                    .to_vec()
                    .try_into()
                    .expect("Unknown should fit in BoundedVec")
            })
        }
    }
}

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests_sprint8;
