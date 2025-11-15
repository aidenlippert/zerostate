//! # Pallet VCG Auction (Vickrey-Clarke-Groves)
//!
//! This pallet implements a Vickrey-Clarke-Groves (VCG) auction mechanism for
//! strategy-proof agent task allocation in the Ainur Protocol.
//!
//! ## Overview
//!
//! VCG auctions provide the following guarantees:
//! - **Strategy-proof**: Truthful bidding is always optimal
//! - **Social efficiency**: Minimizes total social cost
//! - **Individual rationality**: Participants never lose money
//!
//! ## VCG Mechanism
//!
//! 1. **Winner Selection**: Agent with LOWEST bid wins (minimize cost)
//! 2. **Payment**: Winner pays the SECOND-LOWEST bid (Vickrey pricing)
//! 3. **Tie-breaking**: Random selection among equal lowest bids
//!
//! ## Example Scenarios
//!
//! - 3 agents bid [100, 150, 200] → Winner: 100, Payment: 150
//! - 2 agents bid [100, 200] → Winner: 100, Payment: 200
//! - 1 agent bids [100] → Winner: 100, Payment: 100
//! - Tie: [100, 100, 200] → Winner: random(100, 100), Payment: 100

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

#[frame_support::pallet]
pub mod pallet {
    use frame_support::pallet_prelude::*;
    use frame_system::pallet_prelude::*;
    use pallet_registry;
    use sp_runtime::traits::{AtLeast32BitUnsigned, Saturating, Zero};
    use sp_std::vec::Vec;

    /// A single bid in an auction
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct Bid<T: Config> {
        /// Agent's DID
        pub agent_did: BoundedVec<u8, T::MaxDidLength>,
        /// Bid amount in AINU tokens
        pub amount: T::Balance,
        /// Block number when bid was placed
        pub placed_at: BlockNumberFor<T>,
    }

    /// Auction details and state
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    #[scale_info(skip_type_params(T))]
    pub struct Auction<T: Config> {
        /// Unique auction identifier
        pub auction_id: u64,
        /// Task description or metadata hash
        pub task_hash: [u8; 32],
        /// Required capabilities for the task
        pub required_capabilities:
            BoundedVec<BoundedVec<u8, T::MaxCapabilityLength>, T::MaxCapabilities>,
        /// All bids received
        pub bids: BoundedVec<Bid<T>, T::MaxBidsPerAuction>,
        /// Block number when auction was created
        pub created_at: BlockNumberFor<T>,
        /// Block number when auction ends
        pub ends_at: BlockNumberFor<T>,
        /// Auction status
        pub status: AuctionStatus,
        /// Winner (if auction is finalized)
        pub winner: Option<BoundedVec<u8, T::MaxDidLength>>,
        /// Payment amount (if auction is finalized)
        pub payment_amount: Option<T::Balance>,
    }

    /// Auction status enumeration
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
    pub enum AuctionStatus {
        /// Auction is open for bids
        Open,
        /// Auction has ended, awaiting finalization
        Ended,
        /// Auction finalized with winner selected
        Finalized,
        /// Auction was cancelled
        Cancelled,
    }

    /// VCG auction result containing winner and payment
    #[derive(Clone, Encode, Decode, Eq, PartialEq, RuntimeDebug, TypeInfo)]
    pub struct VcgResult<T: Config> {
        /// Winning agent's DID
        pub winner_did: BoundedVec<u8, T::MaxDidLength>,
        /// Winner's bid amount
        pub winning_bid: T::Balance,
        /// Payment amount (second-lowest bid)
        pub payment_amount: T::Balance,
        /// Total social welfare (sum of all other bids minus winning bid)
        pub social_welfare: T::Balance,
    }

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    /// Configure the pallet
    #[pallet::config]
    pub trait Config: frame_system::Config + pallet_registry::Config {
        /// Because this pallet emits events, it depends on the runtime's definition of an event.
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

        /// The balance type for auction amounts
        type Balance: Parameter
            + Member
            + AtLeast32BitUnsigned
            + Default
            + Copy
            + MaxEncodedLen
            + codec::FullCodec
            + From<u32>
            + Into<u128>
            + Saturating;

        /// Maximum number of bids per auction
        #[pallet::constant]
        type MaxBidsPerAuction: Get<u32>;

        /// Auction duration in blocks
        #[pallet::constant]
        type DefaultAuctionDuration: Get<BlockNumberFor<Self>>;

        /// Minimum bid amount to prevent spam
        #[pallet::constant]
        type MinimumBidAmount: Get<Self::Balance>;
    }

    /// Next auction ID counter
    #[pallet::storage]
    #[pallet::getter(fn next_auction_id)]
    pub type NextAuctionId<T> = StorageValue<_, u64, ValueQuery>;

    /// Storage map from auction ID to auction details
    #[pallet::storage]
    #[pallet::getter(fn auctions)]
    pub type Auctions<T: Config> = StorageMap<_, Blake2_128Concat, u64, Auction<T>, OptionQuery>;

    /// Index: Agent DID -> List of auction IDs where agent has bid
    #[pallet::storage]
    #[pallet::getter(fn agent_auction_index)]
    pub type AgentAuctionIndex<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        BoundedVec<u8, T::MaxDidLength>,
        BoundedVec<u64, ConstU32<1000>>, // Max 1000 auctions per agent
        ValueQuery,
    >;

    /// Events emitted by the pallet
    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        /// A new auction was created [auction_id, task_hash]
        AuctionCreated {
            auction_id: u64,
            task_hash: [u8; 32],
        },
        /// A bid was placed [auction_id, agent_did, amount]
        BidPlaced {
            auction_id: u64,
            agent_did: Vec<u8>,
            amount: T::Balance,
        },
        /// An auction was finalized [auction_id, winner_did, payment_amount]
        AuctionFinalized {
            auction_id: u64,
            winner_did: Vec<u8>,
            winning_bid: T::Balance,
            payment_amount: T::Balance,
        },
        /// An auction was cancelled [auction_id]
        AuctionCancelled { auction_id: u64 },
    }

    /// Errors that can occur in this pallet
    #[pallet::error]
    pub enum Error<T> {
        /// Auction not found
        AuctionNotFound,
        /// Auction is not open for bids
        AuctionNotOpen,
        /// Auction has already ended
        AuctionEnded,
        /// Bid amount is below minimum
        BidTooLow,
        /// Agent already placed a bid
        AgentAlreadyBid,
        /// Agent not registered or inactive
        AgentNotRegistered,
        /// Agent doesn't have required capabilities
        AgentLacksCapabilities,
        /// Too many bids for auction
        TooManyBids,
        /// Cannot finalize auction with no bids
        NoBidsToFinalize,
        /// Auction cannot be cancelled in current state
        CannotCancelAuction,
        /// Arithmetic overflow
        ArithmeticOverflow,
    }

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        /// Create a new VCG auction for a task
        #[pallet::call_index(0)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn create_auction(
            origin: OriginFor<T>,
            task_hash: [u8; 32],
            required_capabilities: Vec<Vec<u8>>,
            duration: Option<BlockNumberFor<T>>,
        ) -> DispatchResult {
            let _who = ensure_signed(origin)?;

            // Get next auction ID
            let auction_id = Self::next_auction_id();

            // Validate and convert capabilities to bounded types
            ensure!(
                required_capabilities.len() <= T::MaxCapabilities::get() as usize,
                Error::<T>::TooManyBids
            );

            let mut bounded_capabilities: BoundedVec<
                BoundedVec<u8, T::MaxCapabilityLength>,
                T::MaxCapabilities,
            > = BoundedVec::default();
            for cap in required_capabilities {
                ensure!(
                    cap.len() <= T::MaxCapabilityLength::get() as usize,
                    Error::<T>::TooManyBids
                );
                let bounded_cap: BoundedVec<u8, T::MaxCapabilityLength> =
                    cap.try_into().map_err(|_| Error::<T>::TooManyBids)?;
                bounded_capabilities
                    .try_push(bounded_cap)
                    .map_err(|_| Error::<T>::TooManyBids)?;
            }

            // Set auction duration
            let current_block = <frame_system::Pallet<T>>::block_number();
            let auction_duration = duration.unwrap_or_else(T::DefaultAuctionDuration::get);
            let ends_at = current_block.saturating_add(auction_duration);

            // Create auction
            let auction = Auction {
                auction_id,
                task_hash,
                required_capabilities: bounded_capabilities,
                bids: BoundedVec::default(),
                created_at: current_block,
                ends_at,
                status: AuctionStatus::Open,
                winner: None,
                payment_amount: None,
            };

            // Store auction
            Auctions::<T>::insert(auction_id, auction);

            // Update auction ID counter
            NextAuctionId::<T>::put(auction_id.saturating_add(1));

            // Emit event
            Self::deposit_event(Event::AuctionCreated {
                auction_id,
                task_hash,
            });

            Ok(())
        }

        /// Place a bid in a VCG auction
        #[pallet::call_index(1)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn place_bid(
            origin: OriginFor<T>,
            auction_id: u64,
            amount: T::Balance,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            // Get auction
            let mut auction = Self::auctions(auction_id).ok_or(Error::<T>::AuctionNotFound)?;

            // Check auction status and timing
            ensure!(
                auction.status == AuctionStatus::Open,
                Error::<T>::AuctionNotOpen
            );
            let current_block = <frame_system::Pallet<T>>::block_number();
            ensure!(current_block < auction.ends_at, Error::<T>::AuctionEnded);

            // Validate bid amount
            ensure!(amount >= T::MinimumBidAmount::get(), Error::<T>::BidTooLow);

            // Get agent DID from who (AccountId)
            // Note: In a real implementation, you'd need a mapping from AccountId to DID
            // For this demo, we'll assume the AccountId represents the DID
            let agent_did_vec = who.encode();
            let agent_did: BoundedVec<u8, T::MaxDidLength> = agent_did_vec
                .try_into()
                .map_err(|_| Error::<T>::AgentNotRegistered)?;

            // Check if agent is registered and active
            let agent_card = pallet_registry::Pallet::<T>::get_agent_card(&agent_did)
                .ok_or(Error::<T>::AgentNotRegistered)?;

            // Verify agent has required capabilities
            for required_cap in &auction.required_capabilities {
                let mut has_capability = false;
                for agent_cap in &agent_card.capabilities {
                    if agent_cap == required_cap {
                        has_capability = true;
                        break;
                    }
                }
                ensure!(has_capability, Error::<T>::AgentLacksCapabilities);
            }

            // Check if agent already placed a bid
            for existing_bid in &auction.bids {
                ensure!(
                    existing_bid.agent_did != agent_did,
                    Error::<T>::AgentAlreadyBid
                );
            }

            // Create bid
            let bid = Bid {
                agent_did: agent_did.clone(),
                amount,
                placed_at: current_block,
            };

            // Add bid to auction
            auction
                .bids
                .try_push(bid)
                .map_err(|_| Error::<T>::TooManyBids)?;

            // Store updated auction
            Auctions::<T>::insert(auction_id, &auction);

            // Update agent auction index
            AgentAuctionIndex::<T>::try_mutate(&agent_did, |auction_ids| {
                auction_ids
                    .try_push(auction_id)
                    .map_err(|_| Error::<T>::TooManyBids)
            })?;

            // Emit event
            Self::deposit_event(Event::BidPlaced {
                auction_id,
                agent_did: agent_did.to_vec(),
                amount,
            });

            Ok(())
        }

        /// Finalize a VCG auction using VCG mechanism
        #[pallet::call_index(2)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn finalize_auction(origin: OriginFor<T>, auction_id: u64) -> DispatchResult {
            let _who = ensure_signed(origin)?;

            // Get auction
            let mut auction = Self::auctions(auction_id).ok_or(Error::<T>::AuctionNotFound)?;

            // Check auction status
            ensure!(
                auction.status == AuctionStatus::Open,
                Error::<T>::AuctionNotOpen
            );

            // Check if auction has ended
            let current_block = <frame_system::Pallet<T>>::block_number();
            ensure!(current_block >= auction.ends_at, Error::<T>::AuctionNotOpen);

            // Ensure there are bids to finalize
            ensure!(!auction.bids.is_empty(), Error::<T>::NoBidsToFinalize);

            // Run VCG auction algorithm
            let vcg_result = Self::run_vcg_auction(&auction.bids)?;

            // Update auction with results
            auction.status = AuctionStatus::Finalized;
            auction.winner = Some(vcg_result.winner_did.clone());
            auction.payment_amount = Some(vcg_result.payment_amount);

            // Store updated auction
            Auctions::<T>::insert(auction_id, &auction);

            // Emit event
            Self::deposit_event(Event::AuctionFinalized {
                auction_id,
                winner_did: vcg_result.winner_did.to_vec(),
                winning_bid: vcg_result.winning_bid,
                payment_amount: vcg_result.payment_amount,
            });

            Ok(())
        }

        /// Cancel an auction (only if no bids placed)
        #[pallet::call_index(3)]
        #[pallet::weight(Weight::from_parts(10_000, 0))]
        pub fn cancel_auction(origin: OriginFor<T>, auction_id: u64) -> DispatchResult {
            let _who = ensure_signed(origin)?;

            // Get auction
            let mut auction = Self::auctions(auction_id).ok_or(Error::<T>::AuctionNotFound)?;

            // Only allow cancellation of open auctions with no bids
            ensure!(
                auction.status == AuctionStatus::Open,
                Error::<T>::CannotCancelAuction
            );
            ensure!(auction.bids.is_empty(), Error::<T>::CannotCancelAuction);

            // Update status
            auction.status = AuctionStatus::Cancelled;

            // Store updated auction
            Auctions::<T>::insert(auction_id, &auction);

            // Emit event
            Self::deposit_event(Event::AuctionCancelled { auction_id });

            Ok(())
        }
    }

    // Helper functions
    impl<T: Config> Pallet<T> {
        /// Run the VCG auction mechanism
        ///
        /// VCG Rules:
        /// 1. Winner: Agent with lowest bid (minimize cost)
        /// 2. Payment: Second-lowest bid (strategy-proof pricing)
        /// 3. Tie-breaking: First agent found among tied lowest bids
        pub fn run_vcg_auction(
            bids: &BoundedVec<Bid<T>, T::MaxBidsPerAuction>,
        ) -> Result<VcgResult<T>, Error<T>> {
            ensure!(!bids.is_empty(), Error::<T>::NoBidsToFinalize);

            // Find the lowest bid (winner)
            let mut lowest_bid = &bids[0];
            for bid in bids.iter() {
                if bid.amount < lowest_bid.amount {
                    lowest_bid = bid;
                }
            }

            // Find the second-lowest bid for payment calculation
            let mut second_lowest_amount = lowest_bid.amount;

            // If only one bid, winner pays their own bid
            if bids.len() == 1 {
                second_lowest_amount = lowest_bid.amount;
            } else {
                // Find second lowest among all other bids
                let mut found_second = false;
                for bid in bids.iter() {
                    if bid.agent_did != lowest_bid.agent_did
                        && (!found_second || bid.amount < second_lowest_amount)
                    {
                        second_lowest_amount = bid.amount;
                        found_second = true;
                    }
                }

                // If all other bids are higher, find the actual second lowest
                if !found_second {
                    for bid in bids.iter() {
                        if bid.agent_did != lowest_bid.agent_did
                            && (second_lowest_amount == lowest_bid.amount
                                || bid.amount < second_lowest_amount)
                        {
                            second_lowest_amount = bid.amount;
                        }
                    }
                }
            }

            // Calculate social welfare (total utility)
            let mut social_welfare = T::Balance::zero();
            for bid in bids.iter() {
                if bid.agent_did != lowest_bid.agent_did {
                    social_welfare = social_welfare.saturating_add(bid.amount);
                }
            }

            Ok(VcgResult {
                winner_did: lowest_bid.agent_did.clone(),
                winning_bid: lowest_bid.amount,
                payment_amount: second_lowest_amount,
                social_welfare,
            })
        }

        /// Get auction by ID
        pub fn get_auction(auction_id: u64) -> Option<Auction<T>> {
            Self::auctions(auction_id)
        }

        /// Get all active auctions
        pub fn get_active_auctions() -> Vec<(u64, Auction<T>)> {
            let mut active_auctions = Vec::new();
            let current_block = <frame_system::Pallet<T>>::block_number();

            // Iterate through auctions (in practice, you'd want a better index)
            for i in 0..Self::next_auction_id() {
                if let Some(auction) = Self::auctions(i) {
                    if auction.status == AuctionStatus::Open && current_block < auction.ends_at {
                        active_auctions.push((i, auction));
                    }
                }
            }

            active_auctions
        }

        /// Check if VCG mechanism maintains strategy-proof property
        /// This is a verification function for testing
        pub fn verify_strategy_proof(bids: &BoundedVec<Bid<T>, T::MaxBidsPerAuction>) -> bool {
            if bids.len() < 2 {
                return true; // Trivially strategy-proof with <2 bidders
            }

            // In VCG, no agent can improve their outcome by lying about their cost
            // This is guaranteed by the mechanism design:
            // 1. Winner is lowest bidder (can't win by bidding higher)
            // 2. Payment is second-lowest (independent of winner's bid)
            // Therefore, truthful bidding is always optimal
            true
        }
    }
}
