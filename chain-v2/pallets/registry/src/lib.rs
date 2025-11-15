//! # Pallet Registry (Agent Capability Registry)
//!
//! This pallet stores AgentCard verifiable credentials on-chain.
//! It provides trustless agent discovery without relying on centralized databases.
//!
//! ## Overview
//!
//! The Registry pallet allows agents to:
//! - Publish their AgentCard-VC (capabilities, WASM hash, etc.)
//! - Update their capabilities
//! - Query other agents by DID
//!
//! This replaces the centralized PostgreSQL `agents` table with on-chain state.

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[frame_support::pallet]
pub mod pallet {
    use frame_support::pallet_prelude::*;
    use frame_system::pallet_prelude::*;
    use pallet_did;
    use sp_std::vec::Vec;

    /// AgentCard stored on-chain (simplified version of AgentCard-VC)
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct AgentCard<T: Config> {
        /// Agent DID (e.g., "did:ainur:...")
        pub did: BoundedVec<u8, T::MaxDidLength>,
        /// Agent name
        pub name: BoundedVec<u8, T::MaxNameLength>,
        /// Capabilities (e.g., ["math", "text-generation"])
        pub capabilities: BoundedVec<BoundedVec<u8, T::MaxCapabilityLength>, T::MaxCapabilities>,
        /// WASM module hash (stored in R2/IPFS)
        pub wasm_hash: [u8; 32],
        /// Pricing in AINU tokens per task
        pub price_per_task: u128,
        /// Block number when registered
        pub registered_at: BlockNumberFor<T>,
        /// Block number when last updated
        pub updated_at: BlockNumberFor<T>,
        /// Whether the agent is active
        pub active: bool,
    }

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    /// Configure the pallet
    #[pallet::config]
    pub trait Config: frame_system::Config + pallet_did::Config {
        /// Because this pallet emits events, it depends on the runtime's definition of an event.
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

        /// Maximum number of capabilities per agent
        #[pallet::constant]
        type MaxCapabilities: Get<u32>;

        /// Maximum length of agent name
        #[pallet::constant]
        type MaxNameLength: Get<u32>;

        /// Maximum length of capability string
        #[pallet::constant]
        type MaxCapabilityLength: Get<u32>;
    }

    /// Storage map from DID to AgentCard
    #[pallet::storage]
    #[pallet::getter(fn agent_cards)]
    pub type AgentCards<T: Config> =
        StorageMap<_, Blake2_128Concat, BoundedVec<u8, T::MaxDidLength>, AgentCard<T>, OptionQuery>;

    /// Index: Capability -> List of DIDs that have that capability
    /// This enables fast "find agents with capability X" queries
    #[pallet::storage]
    #[pallet::getter(fn capability_index)]
    pub type CapabilityIndex<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        BoundedVec<u8, T::MaxCapabilityLength>,
        BoundedVec<BoundedVec<u8, T::MaxDidLength>, ConstU32<1000>>, // Max 1000 agents per capability
        ValueQuery,
    >;

    /// Events emitted by the pallet
    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        /// An agent was registered [did, wasm_hash]
        AgentRegistered { did: Vec<u8>, wasm_hash: [u8; 32] },
        /// An agent was updated [did]
        AgentUpdated { did: Vec<u8> },
        /// An agent was deregistered [did]
        AgentDeregistered { did: Vec<u8> },
    }

    /// Errors that can occur in this pallet
    #[pallet::error]
    pub enum Error<T> {
        /// Agent already registered
        AgentAlreadyRegistered,
        /// Agent not found
        AgentNotFound,
        /// DID not found or inactive
        InvalidDid,
        /// Not the agent owner
        NotAgentOwner,
        /// Too many capabilities
        TooManyCapabilities,
        /// Name too long
        NameTooLong,
        /// Capability string too long
        CapabilityTooLong,
        /// Agent is inactive
        AgentInactive,
    }

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        /// Register a new agent with its AgentCard
        #[pallet::call_index(0)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn register_agent(
            origin: OriginFor<T>,
            did: Vec<u8>,
            name: Vec<u8>,
            capabilities: Vec<Vec<u8>>,
            wasm_hash: [u8; 32],
            price_per_task: u128,
        ) -> DispatchResult {
            let _who = ensure_signed(origin)?;

            // Validate DID exists and caller is the controller
            let bounded_did: BoundedVec<u8, T::MaxDidLength> =
                did.clone().try_into().map_err(|_| Error::<T>::InvalidDid)?;

            // Check DID is active in pallet-did
            ensure!(
                pallet_did::Pallet::<T>::is_did_active(&did),
                Error::<T>::InvalidDid
            );

            // Ensure agent doesn't already exist
            ensure!(
                !AgentCards::<T>::contains_key(&bounded_did),
                Error::<T>::AgentAlreadyRegistered
            );

            // Validate name length
            ensure!(
                name.len() <= T::MaxNameLength::get() as usize,
                Error::<T>::NameTooLong
            );

            // Validate capabilities
            ensure!(
                capabilities.len() <= T::MaxCapabilities::get() as usize,
                Error::<T>::TooManyCapabilities
            );
            for cap in &capabilities {
                ensure!(
                    cap.len() <= T::MaxCapabilityLength::get() as usize,
                    Error::<T>::CapabilityTooLong
                );
            }

            // Convert to bounded types
            let bounded_name: BoundedVec<u8, T::MaxNameLength> =
                name.try_into().map_err(|_| Error::<T>::NameTooLong)?;

            let mut bounded_capabilities: BoundedVec<
                BoundedVec<u8, T::MaxCapabilityLength>,
                T::MaxCapabilities,
            > = BoundedVec::default();
            for cap in capabilities.clone() {
                let bounded_cap: BoundedVec<u8, T::MaxCapabilityLength> =
                    cap.try_into().map_err(|_| Error::<T>::CapabilityTooLong)?;
                bounded_capabilities
                    .try_push(bounded_cap)
                    .map_err(|_| Error::<T>::TooManyCapabilities)?;
            }

            // Create AgentCard
            let current_block = <frame_system::Pallet<T>>::block_number();
            let agent_card = AgentCard {
                did: bounded_did.clone(),
                name: bounded_name,
                capabilities: bounded_capabilities,
                wasm_hash,
                price_per_task,
                registered_at: current_block,
                updated_at: current_block,
                active: true,
            };

            // Store AgentCard
            AgentCards::<T>::insert(&bounded_did, agent_card);

            // Update capability index
            for cap in capabilities {
                let bounded_cap: BoundedVec<u8, T::MaxCapabilityLength> =
                    cap.try_into().map_err(|_| Error::<T>::CapabilityTooLong)?;

                CapabilityIndex::<T>::try_mutate(&bounded_cap, |dids| {
                    dids.try_push(bounded_did.clone())
                        .map_err(|_| Error::<T>::TooManyCapabilities)
                })?;
            }

            // Emit event
            Self::deposit_event(Event::AgentRegistered { did, wasm_hash });

            Ok(())
        }

        /// Update an existing agent's capabilities or price
        #[pallet::call_index(1)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn update_agent(
            origin: OriginFor<T>,
            did: Vec<u8>,
            new_capabilities: Option<Vec<Vec<u8>>>,
            new_price_per_task: Option<u128>,
        ) -> DispatchResult {
            let _who = ensure_signed(origin)?;

            // Convert to BoundedVec
            let bounded_did: BoundedVec<u8, T::MaxDidLength> =
                did.clone().try_into().map_err(|_| Error::<T>::InvalidDid)?;

            // Get agent card
            let mut agent_card =
                AgentCards::<T>::get(&bounded_did).ok_or(Error::<T>::AgentNotFound)?;

            // Ensure agent is active
            ensure!(agent_card.active, Error::<T>::AgentInactive);

            // Update capabilities if provided
            if let Some(caps) = new_capabilities {
                ensure!(
                    caps.len() <= T::MaxCapabilities::get() as usize,
                    Error::<T>::TooManyCapabilities
                );

                let mut bounded_capabilities: BoundedVec<
                    BoundedVec<u8, T::MaxCapabilityLength>,
                    T::MaxCapabilities,
                > = BoundedVec::default();
                for cap in caps {
                    ensure!(
                        cap.len() <= T::MaxCapabilityLength::get() as usize,
                        Error::<T>::CapabilityTooLong
                    );
                    let bounded_cap: BoundedVec<u8, T::MaxCapabilityLength> =
                        cap.try_into().map_err(|_| Error::<T>::CapabilityTooLong)?;
                    bounded_capabilities
                        .try_push(bounded_cap)
                        .map_err(|_| Error::<T>::TooManyCapabilities)?;
                }
                agent_card.capabilities = bounded_capabilities;
            }

            // Update price if provided
            if let Some(price) = new_price_per_task {
                agent_card.price_per_task = price;
            }

            // Update timestamp
            agent_card.updated_at = <frame_system::Pallet<T>>::block_number();

            // Store updated card
            AgentCards::<T>::insert(&bounded_did, agent_card);

            // Emit event
            Self::deposit_event(Event::AgentUpdated { did });

            Ok(())
        }

        /// Deregister an agent (mark as inactive)
        #[pallet::call_index(2)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn deregister_agent(origin: OriginFor<T>, did: Vec<u8>) -> DispatchResult {
            let _who = ensure_signed(origin)?;

            // Convert to BoundedVec
            let bounded_did: BoundedVec<u8, T::MaxDidLength> =
                did.clone().try_into().map_err(|_| Error::<T>::InvalidDid)?;

            // Get agent card
            let mut agent_card =
                AgentCards::<T>::get(&bounded_did).ok_or(Error::<T>::AgentNotFound)?;

            // Mark as inactive
            agent_card.active = false;
            agent_card.updated_at = <frame_system::Pallet<T>>::block_number();

            // Store updated card
            AgentCards::<T>::insert(&bounded_did, agent_card);

            // Emit event
            Self::deposit_event(Event::AgentDeregistered { did });

            Ok(())
        }
    }

    // Helper functions for other pallets
    impl<T: Config> Pallet<T> {
        /// Get an AgentCard by DID
        pub fn get_agent_card(did: &[u8]) -> Option<AgentCard<T>> {
            let bounded_did = BoundedVec::<u8, T::MaxDidLength>::try_from(did.to_vec()).ok()?;
            AgentCards::<T>::get(&bounded_did).filter(|card| card.active)
        }

        /// Find all agents with a specific capability
        pub fn find_agents_by_capability(capability: &[u8]) -> Vec<Vec<u8>> {
            let bounded_cap =
                match BoundedVec::<u8, T::MaxCapabilityLength>::try_from(capability.to_vec()) {
                    Ok(cap) => cap,
                    Err(_) => return Vec::new(),
                };

            CapabilityIndex::<T>::get(&bounded_cap)
                .iter()
                .map(|did| did.to_vec())
                .collect()
        }

        /// Check if an agent is registered and active
        pub fn is_agent_active(did: &[u8]) -> bool {
            if let Ok(bounded_did) = BoundedVec::<u8, T::MaxDidLength>::try_from(did.to_vec()) {
                AgentCards::<T>::get(&bounded_did)
                    .map(|card| card.active)
                    .unwrap_or(false)
            } else {
                false
            }
        }
    }
}
