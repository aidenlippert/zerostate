//! # Reputation Pallet
//!
//! FAANG-level reputation system with staking and slashing for Ainur Protocol.
//!
//! Based on Substrate's pallet-staking patterns and 2024 NPoS research.
//!
//! ## Features
//!
//! - **Reputation Staking**: Agents bond AINU tokens to gain reputation
//! - **Task-Based Rewards**: Reputation increases on successful task completion
//! - **Proportional Slashing**: Failed tasks slash 1%, fraud slashes up to 50%
//! - **Logarithmic Growth**: Prevents reputation inflation
//! - **Economic Security**: Aligned incentives via stake at risk
//!
//! ## Economic Parameters
//!
//! - Min stake: 100 AINU
//! - Starting reputation: 500 (out of 1000)
//! - Slash rates: 1% (failed task) → 50% (fraud)
//! - Reputation growth: Logarithmic (diminishing returns)

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[frame_support::pallet]
pub mod pallet {
    use frame_support::{
        pallet_prelude::*,
        traits::{Currency, ExistenceRequirement, ReservableCurrency},
    };
    use frame_system::pallet_prelude::*;
    use sp_runtime::traits::{Saturating, Zero};
    use sp_std::vec::Vec;

    type BalanceOf<T> =
        <<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    /// Reputation stake information for an agent
    #[derive(Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct ReputationStake<T: Config> {
        /// Staked AINU tokens
        pub staked: BalanceOf<T>,
        /// Reputation score (0-1000)
        pub reputation: u32,
        /// Tasks completed successfully
        pub tasks_completed: u32,
        /// Tasks failed
        pub tasks_failed: u32,
        /// Total slashed amount
        pub slashed: BalanceOf<T>,
        /// Block number when stake was created
        pub active_since: BlockNumberFor<T>,
    }

    impl<T: Config> Clone for ReputationStake<T> {
        fn clone(&self) -> Self {
            Self {
                staked: self.staked,
                reputation: self.reputation,
                tasks_completed: self.tasks_completed,
                tasks_failed: self.tasks_failed,
                slashed: self.slashed,
                active_since: self.active_since,
            }
        }
    }

    /// Offense types for severe slashing
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    #[codec(encode_bound())]
    #[codec(decode_bound())]
    pub enum OffenseType {
        /// Fraudulent task result (50% slash)
        FraudulentResult,
        /// Accepted multiple tasks simultaneously when capacity full (30% slash)
        DoubleTaskAcceptance,
        /// Repeated failures in short time period (25% slash)
        RepeatedFailures,
        /// Protocol violation (20% slash)
        ProtocolViolation,
    }

    /// Configuration trait for reputation pallet
    #[pallet::config]
    pub trait Config: frame_system::Config {
        /// The overarching event type
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

        /// Currency type for staking
        type Currency: ReservableCurrency<Self::AccountId>;

        /// Minimum reputation stake (100 AINU)
        #[pallet::constant]
        type MinReputationStake: Get<BalanceOf<Self>>;

        /// Maximum reputation score (1000)
        #[pallet::constant]
        type MaxReputationScore: Get<u32>;

        /// Origin that can report task outcomes (orchestrators)
        type OrchestratorOrigin: EnsureOrigin<Self::RuntimeOrigin>;

        /// Origin that can slash for severe offenses (governance)
        type SlashingOrigin: EnsureOrigin<Self::RuntimeOrigin>;

        /// Treasury account for slashed funds
        type TreasuryAccount: Get<Self::AccountId>;
    }

    /// Storage: Agent DID → Reputation stake info
    #[pallet::storage]
    #[pallet::getter(fn reputation_stake)]
    pub type ReputationStakes<T: Config> =
        StorageMap<_, Blake2_128Concat, T::AccountId, ReputationStake<T>, OptionQuery>;

    /// Events emitted by reputation pallet
    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        /// Reputation stake bonded [agent, amount]
        ReputationBonded(T::AccountId, BalanceOf<T>),
        /// Reputation stake unbonded [agent, amount]
        ReputationUnbonded(T::AccountId, BalanceOf<T>),
        /// Task outcome reported [agent, task_id, success]
        TaskOutcomeReported(T::AccountId, Vec<u8>, bool),
        /// Reputation increased [agent, old_score, new_score]
        ReputationIncreased(T::AccountId, u32, u32),
        /// Reputation decreased [agent, old_score, new_score, slashed_amount]
        ReputationDecreased(T::AccountId, u32, u32, BalanceOf<T>),
        /// Severe slash applied [agent, slash_percentage]
        SevereSlash(T::AccountId, u32),
    }

    /// Errors for reputation pallet
    #[pallet::error]
    pub enum Error<T> {
        /// Stake amount below minimum
        StakeTooLow,
        /// No stake exists for agent
        NoStake,
        /// Insufficient staked funds
        InsufficientStake,
        /// Reputation already at maximum
        ReputationAtMax,
        /// Reputation already at minimum
        ReputationAtMin,
    }

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        /// Bond reputation stake
        ///
        /// Agents bond AINU tokens to participate in the network and build reputation.
        ///
        /// Parameters:
        /// - `origin`: Agent account
        /// - `value`: Amount of AINU to stake (must be >= MinReputationStake)
        ///
        /// Emits: `ReputationBonded`
        #[pallet::call_index(0)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn bond_reputation(
            origin: OriginFor<T>,
            #[pallet::compact] value: BalanceOf<T>,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            // Validate minimum stake
            ensure!(
                value >= T::MinReputationStake::get(),
                Error::<T>::StakeTooLow
            );

            // Reserve funds from agent account
            T::Currency::reserve(&who, value)?;

            // Initialize or update stake
            let current_block = <frame_system::Pallet<T>>::block_number();
            let stake = ReputationStakes::<T>::get(&who).unwrap_or(ReputationStake {
                staked: Zero::zero(),
                reputation: 0,
                tasks_completed: 0,
                tasks_failed: 0,
                slashed: Zero::zero(),
                active_since: current_block,
            });

            let new_stake = ReputationStake {
                staked: stake.staked.saturating_add(value),
                reputation: if stake.reputation == 0 {
                    500
                } else {
                    stake.reputation
                },
                tasks_completed: stake.tasks_completed,
                tasks_failed: stake.tasks_failed,
                slashed: stake.slashed,
                active_since: if stake.reputation == 0 {
                    current_block
                } else {
                    stake.active_since
                },
            };

            ReputationStakes::<T>::insert(&who, new_stake);

            Self::deposit_event(Event::ReputationBonded(who, value));
            Ok(())
        }

        /// Unbond reputation stake
        ///
        /// Agents can unbond staked tokens. Reputation is preserved but no new reputation
        /// can be earned without active stake.
        ///
        /// Parameters:
        /// - `origin`: Agent account
        /// - `value`: Amount of AINU to unbond
        ///
        /// Emits: `ReputationUnbonded`
        #[pallet::call_index(1)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn unbond_reputation(
            origin: OriginFor<T>,
            #[pallet::compact] value: BalanceOf<T>,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            // Check stake exists
            let mut stake = ReputationStakes::<T>::get(&who).ok_or(Error::<T>::NoStake)?;
            ensure!(stake.staked >= value, Error::<T>::InsufficientStake);

            // Unreserve funds
            T::Currency::unreserve(&who, value);

            // Update stake
            stake.staked = stake.staked.saturating_sub(value);
            ReputationStakes::<T>::insert(&who, stake);

            Self::deposit_event(Event::ReputationUnbonded(who, value));
            Ok(())
        }

        /// Report task outcome (orchestrator only)
        ///
        /// Orchestrators call this after task completion to update agent reputation.
        ///
        /// Parameters:
        /// - `origin`: Orchestrator account
        /// - `agent`: Agent DID
        /// - `task_id`: Task identifier
        /// - `success`: True if task completed successfully
        ///
        /// Emits: `TaskOutcomeReported`, `ReputationIncreased` or `ReputationDecreased`
        #[pallet::call_index(2)]
        #[pallet::weight(Weight::from_parts(5_000, 0))]
        pub fn report_outcome(
            origin: OriginFor<T>,
            agent: T::AccountId,
            task_id: Vec<u8>,
            success: bool,
        ) -> DispatchResult {
            // Only orchestrators can report
            T::OrchestratorOrigin::ensure_origin(origin)?;

            let mut stake = ReputationStakes::<T>::get(&agent).ok_or(Error::<T>::NoStake)?;

            let old_reputation = stake.reputation;

            if success {
                // Increase reputation (logarithmic growth)
                stake.tasks_completed = stake.tasks_completed.saturating_add(1);

                // Reputation gain decreases as reputation increases
                let reputation_gain = 10u32.saturating_sub(stake.reputation / 100);
                let new_reputation = stake
                    .reputation
                    .saturating_add(reputation_gain)
                    .min(T::MaxReputationScore::get());

                stake.reputation = new_reputation;

                ReputationStakes::<T>::insert(&agent, stake);

                Self::deposit_event(Event::ReputationIncreased(
                    agent.clone(),
                    old_reputation,
                    new_reputation,
                ));
            } else {
                // Decrease reputation + slash
                stake.tasks_failed = stake.tasks_failed.saturating_add(1);

                let reputation_loss = 20u32;
                let new_reputation = stake.reputation.saturating_sub(reputation_loss);
                stake.reputation = new_reputation;

                // Slash 1% of stake per failed task
                let slash_amount = stake.staked / 100u32.into();
                stake.staked = stake.staked.saturating_sub(slash_amount);
                stake.slashed = stake.slashed.saturating_add(slash_amount);

                // Transfer slashed funds to treasury
                T::Currency::unreserve(&agent, slash_amount);
                T::Currency::transfer(
                    &agent,
                    &T::TreasuryAccount::get(),
                    slash_amount,
                    ExistenceRequirement::AllowDeath,
                )?;

                ReputationStakes::<T>::insert(&agent, stake);

                Self::deposit_event(Event::ReputationDecreased(
                    agent.clone(),
                    old_reputation,
                    new_reputation,
                    slash_amount,
                ));
            }

            Self::deposit_event(Event::TaskOutcomeReported(agent, task_id, success));
            Ok(())
        }

        /// Slash for severe misbehavior (governance only)
        ///
        /// Governance can slash agents for severe offenses like fraud.
        ///
        /// Parameters:
        /// - `origin`: Governance account
        /// - `agent`: Agent to slash
        /// - `offense_code`: Offense type code (0=FraudulentResult, 1=DoubleTaskAcceptance, 2=RepeatedFailures, 3=ProtocolViolation)
        ///
        /// Emits: `SevereSlash`, `ReputationDecreased`
        #[pallet::call_index(3)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn slash_severe(
            origin: OriginFor<T>,
            agent: T::AccountId,
            offense_code: u8,
        ) -> DispatchResult {
            T::SlashingOrigin::ensure_origin(origin)?;

            let mut stake = ReputationStakes::<T>::get(&agent).ok_or(Error::<T>::NoStake)?;

            // Determine slash percentage based on offense code
            // 0=FraudulentResult(50%), 1=DoubleTaskAcceptance(30%), 2=RepeatedFailures(25%), 3=ProtocolViolation(20%)
            let slash_percentage = match offense_code {
                0 => 50, // FraudulentResult
                1 => 30, // DoubleTaskAcceptance
                2 => 25, // RepeatedFailures
                3 => 20, // ProtocolViolation
                _ => 20, // Default to lowest slash
            };

            let old_reputation = stake.reputation;

            // Slash stake
            let slash_amount = stake.staked * slash_percentage.into() / 100u32.into();
            stake.staked = stake.staked.saturating_sub(slash_amount);
            stake.slashed = stake.slashed.saturating_add(slash_amount);

            // Zero reputation on severe offense
            stake.reputation = 0;

            // Transfer slashed funds to treasury
            T::Currency::unreserve(&agent, slash_amount);
            T::Currency::transfer(
                &agent,
                &T::TreasuryAccount::get(),
                slash_amount,
                ExistenceRequirement::AllowDeath,
            )?;

            ReputationStakes::<T>::insert(&agent, stake);

            Self::deposit_event(Event::SevereSlash(agent.clone(), slash_percentage));
            Self::deposit_event(Event::ReputationDecreased(
                agent,
                old_reputation,
                0,
                slash_amount,
            ));

            Ok(())
        }
    }
}
