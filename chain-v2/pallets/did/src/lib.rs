//! # Pallet DID (Decentralized Identifiers)
//!
//! This pallet implements on-chain DID management for Ainur agents.
//! It provides trustless identity without relying on centralized databases.
//!
//! ## Overview
//!
//! The DID pallet allows agents to:
//! - Create did:ainur identifiers
//! - Register Ed25519 public keys
//! - Update DID documents
//! - Resolve DIDs to their associated public keys
//!
//! ## Interface
//!
//! ### Dispatchable Functions
//!
//! - `create_did` - Create a new DID with a public key
//! - `update_key` - Update the public key associated with a DID
//! - `revoke_did` - Revoke a DID (mark as inactive)
//!
//! ### Public Functions
//!
//! - `resolve_did` - Get the public key for a given DID
//! - `verify_signature` - Verify a signature against a DID's public key

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[frame_support::pallet]
pub mod pallet {
    use frame_support::pallet_prelude::*;
    use frame_system::pallet_prelude::*;
    use sp_std::vec::Vec;

    /// DID Document stored on-chain
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct DidDocument<AccountId, BlockNumber> {
        /// The DID controller (account that owns this DID)
        pub controller: AccountId,
        /// Ed25519 public key for verification
        pub public_key: [u8; 32],
        /// Block number when DID was created
        pub created_at: BlockNumber,
        /// Block number when DID was last updated
        pub updated_at: BlockNumber,
        /// Whether the DID is active
        pub active: bool,
    }

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    /// Configure the pallet by specifying the parameters and types it depends on.
    #[pallet::config]
    pub trait Config: frame_system::Config {
        /// Because this pallet emits events, it depends on the runtime's definition of an event.
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

        /// Maximum length of a DID identifier (e.g., "did:ainur:...")
        #[pallet::constant]
        type MaxDidLength: Get<u32>;
    }

    /// Storage map from DID string to DID Document
    #[pallet::storage]
    #[pallet::getter(fn did_documents)]
    pub type DidDocuments<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        BoundedVec<u8, T::MaxDidLength>,
        DidDocument<T::AccountId, BlockNumberFor<T>>,
        OptionQuery,
    >;

    /// Reverse lookup: Account ID to DID
    #[pallet::storage]
    #[pallet::getter(fn account_to_did)]
    pub type AccountToDid<T: Config> =
        StorageMap<_, Blake2_128Concat, T::AccountId, BoundedVec<u8, T::MaxDidLength>, OptionQuery>;

    /// Events emitted by the pallet
    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        /// A new DID was created [did, controller, public_key]
        DidCreated {
            did: Vec<u8>,
            controller: T::AccountId,
            public_key: [u8; 32],
        },
        /// A DID's public key was updated [did, new_public_key]
        DidUpdated {
            did: Vec<u8>,
            new_public_key: [u8; 32],
        },
        /// A DID was revoked [did]
        DidRevoked { did: Vec<u8> },
    }

    /// Errors that can occur in this pallet
    #[pallet::error]
    pub enum Error<T> {
        /// DID already exists
        DidAlreadyExists,
        /// DID not found
        DidNotFound,
        /// Not the DID controller
        NotDidController,
        /// DID is inactive/revoked
        DidInactive,
        /// Invalid DID format
        InvalidDidFormat,
        /// DID length exceeds maximum
        DidTooLong,
    }

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        /// Create a new DID
        ///
        /// Parameters:
        /// - `did`: The DID identifier (e.g., "did:ainur:...")
        /// - `public_key`: Ed25519 public key (32 bytes)
        #[pallet::call_index(0)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn create_did(
            origin: OriginFor<T>,
            did: Vec<u8>,
            public_key: [u8; 32],
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            // Validate DID format
            ensure!(did.starts_with(b"did:ainur:"), Error::<T>::InvalidDidFormat);

            // Convert to BoundedVec
            let bounded_did: BoundedVec<u8, T::MaxDidLength> =
                did.clone().try_into().map_err(|_| Error::<T>::DidTooLong)?;

            // Ensure DID doesn't already exist
            ensure!(
                !DidDocuments::<T>::contains_key(&bounded_did),
                Error::<T>::DidAlreadyExists
            );

            // Create DID document
            let current_block = <frame_system::Pallet<T>>::block_number();
            let did_doc = DidDocument {
                controller: who.clone(),
                public_key,
                created_at: current_block,
                updated_at: current_block,
                active: true,
            };

            // Store DID document
            DidDocuments::<T>::insert(&bounded_did, did_doc);

            // Store reverse lookup
            AccountToDid::<T>::insert(&who, &bounded_did);

            // Emit event
            Self::deposit_event(Event::DidCreated {
                did,
                controller: who,
                public_key,
            });

            Ok(())
        }

        /// Update a DID's public key
        ///
        /// Only the DID controller can update the key
        #[pallet::call_index(1)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn update_key(
            origin: OriginFor<T>,
            did: Vec<u8>,
            new_public_key: [u8; 32],
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            // Convert to BoundedVec
            let bounded_did: BoundedVec<u8, T::MaxDidLength> =
                did.clone().try_into().map_err(|_| Error::<T>::DidTooLong)?;

            // Get DID document
            let mut did_doc =
                DidDocuments::<T>::get(&bounded_did).ok_or(Error::<T>::DidNotFound)?;

            // Ensure caller is the controller
            ensure!(did_doc.controller == who, Error::<T>::NotDidController);

            // Ensure DID is active
            ensure!(did_doc.active, Error::<T>::DidInactive);

            // Update key and timestamp
            did_doc.public_key = new_public_key;
            did_doc.updated_at = <frame_system::Pallet<T>>::block_number();

            // Store updated document
            DidDocuments::<T>::insert(&bounded_did, did_doc);

            // Emit event
            Self::deposit_event(Event::DidUpdated {
                did,
                new_public_key,
            });

            Ok(())
        }

        /// Revoke a DID (mark as inactive)
        ///
        /// Only the DID controller can revoke
        #[pallet::call_index(2)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn revoke_did(origin: OriginFor<T>, did: Vec<u8>) -> DispatchResult {
            let who = ensure_signed(origin)?;

            // Convert to BoundedVec
            let bounded_did: BoundedVec<u8, T::MaxDidLength> =
                did.clone().try_into().map_err(|_| Error::<T>::DidTooLong)?;

            // Get DID document
            let mut did_doc =
                DidDocuments::<T>::get(&bounded_did).ok_or(Error::<T>::DidNotFound)?;

            // Ensure caller is the controller
            ensure!(did_doc.controller == who, Error::<T>::NotDidController);

            // Mark as inactive
            did_doc.active = false;
            did_doc.updated_at = <frame_system::Pallet<T>>::block_number();

            // Store updated document
            DidDocuments::<T>::insert(&bounded_did, did_doc);

            // Emit event
            Self::deposit_event(Event::DidRevoked { did });

            Ok(())
        }
    }

    // Helper functions for other pallets
    impl<T: Config> Pallet<T> {
        /// Resolve a DID to its public key (for signature verification)
        pub fn resolve_public_key(did: &[u8]) -> Option<[u8; 32]> {
            let bounded_did = BoundedVec::<u8, T::MaxDidLength>::try_from(did.to_vec()).ok()?;
            DidDocuments::<T>::get(&bounded_did)
                .filter(|doc| doc.active)
                .map(|doc| doc.public_key)
        }

        /// Check if a DID exists and is active
        pub fn is_did_active(did: &[u8]) -> bool {
            if let Ok(bounded_did) = BoundedVec::<u8, T::MaxDidLength>::try_from(did.to_vec()) {
                DidDocuments::<T>::get(&bounded_did)
                    .map(|doc| doc.active)
                    .unwrap_or(false)
            } else {
                false
            }
        }
    }
}
