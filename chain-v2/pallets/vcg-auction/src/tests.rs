use crate::{mock::*, AuctionStatus, Bid, Error, Event};
use codec::Encode;
use frame_support::{assert_noop, assert_ok, traits::ConstU32, BoundedVec};
use frame_system::RawOrigin;

/// Helper function to register a test agent
fn register_test_agent(account: u64, _did: &[u8], capabilities: Vec<Vec<u8>>) {
    // VCG auction expects DID to be the encoded AccountId
    // So we need to register the agent with the AccountId as DID
    let agent_did_vec = account.encode();

    // First create DID - need to format it properly for DID pallet
    let mut proper_did = b"did:ainur:account".to_vec();
    // Append the account ID to make it unique
    proper_did.extend_from_slice(&account.to_string().as_bytes());

    assert_ok!(Did::create_did(
        RawOrigin::Signed(account).into(),
        proper_did.clone(),
        [1u8; 32], // public key as [u8; 32]
    ));

    // Then register agent in registry with the encoded AccountId as DID
    // This is what VCG auction looks for: the encoded AccountId
    assert_ok!(Registry::register_agent(
        RawOrigin::Signed(account).into(),
        agent_did_vec,
        b"Test Agent".to_vec(),
        capabilities,
        [0u8; 32], // wasm_hash
        100,       // price_per_task
    ));
}

/// Helper function to create a test auction
fn create_test_auction() -> u64 {
    let task_hash = [1u8; 32];
    let capabilities = vec![b"math".to_vec()];

    assert_ok!(VcgAuction::create_auction(
        RawOrigin::Signed(1).into(),
        task_hash,
        capabilities,
        None, // Use default duration
    ));

    0 // First auction ID
}

#[test]
fn create_auction_works() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        let task_hash = [1u8; 32];
        let capabilities = vec![b"math".to_vec(), b"text".to_vec()];

        // Create auction
        assert_ok!(VcgAuction::create_auction(
            RawOrigin::Signed(1).into(),
            task_hash,
            capabilities.clone(),
            Some(200), // Custom duration
        ));

        // Check that auction was created
        let auction = VcgAuction::auctions(0).unwrap();
        assert_eq!(auction.auction_id, 0);
        assert_eq!(auction.task_hash, task_hash);
        assert_eq!(auction.status, AuctionStatus::Open);
        assert_eq!(auction.bids.len(), 0);

        // Check capabilities were stored correctly
        assert_eq!(auction.required_capabilities.len(), 2);

        // Check next auction ID was incremented
        assert_eq!(VcgAuction::next_auction_id(), 1);

        // Check event was emitted
        System::assert_last_event(
            Event::AuctionCreated {
                auction_id: 0,
                task_hash,
            }
            .into(),
        );
    });
}

#[test]
fn place_bid_works() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Setup: Register agents and create auction
        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);
        let auction_id = create_test_auction();

        // Place bid
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(2).into(),
            auction_id,
            150,
        ));

        // Check bid was recorded
        let auction = VcgAuction::auctions(auction_id).unwrap();
        assert_eq!(auction.bids.len(), 1);
        assert_eq!(auction.bids[0].amount, 150);

        // Check event was emitted
        System::assert_last_event(
            Event::BidPlaced {
                auction_id,
                agent_did: 2u64.to_be_bytes().to_vec(),
                amount: 150,
            }
            .into(),
        );
    });
}

#[test]
fn place_bid_fails_for_unregistered_agent() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        let auction_id = create_test_auction();

        // Try to place bid without registering agent
        assert_noop!(
            VcgAuction::place_bid(RawOrigin::Signed(2).into(), auction_id, 150),
            Error::<Test>::AgentNotRegistered
        );
    });
}

#[test]
fn place_bid_fails_for_insufficient_capabilities() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Register agent without required capability
        register_test_agent(2, b"agent1", vec![b"text".to_vec()]);
        let auction_id = create_test_auction();

        // Try to place bid (agent lacks 'math' capability)
        assert_noop!(
            VcgAuction::place_bid(RawOrigin::Signed(2).into(), auction_id, 150),
            Error::<Test>::AgentLacksCapabilities
        );
    });
}

#[test]
fn place_bid_fails_for_duplicate_bid() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Setup
        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);
        let auction_id = create_test_auction();

        // Place first bid
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(2).into(),
            auction_id,
            150
        ));

        // Try to place second bid from same agent
        assert_noop!(
            VcgAuction::place_bid(RawOrigin::Signed(2).into(), auction_id, 120),
            Error::<Test>::AgentAlreadyBid
        );
    });
}

#[test]
fn place_bid_fails_for_low_amount() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);
        let auction_id = create_test_auction();

        // Try to place bid below minimum
        assert_noop!(
            VcgAuction::place_bid(RawOrigin::Signed(2).into(), auction_id, 5),
            Error::<Test>::BidTooLow
        );
    });
}

#[test]
fn vcg_auction_three_bidders_scenario() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Setup: Register three agents
        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);
        register_test_agent(3, b"agent2", vec![b"math".to_vec()]);
        register_test_agent(4, b"agent3", vec![b"math".to_vec()]);

        let auction_id = create_test_auction();

        // Place bids: [100, 150, 200]
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(2).into(),
            auction_id,
            100
        ));
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(3).into(),
            auction_id,
            150
        ));
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(4).into(),
            auction_id,
            200
        ));

        // Fast forward past auction end
        System::set_block_number(101);

        // Finalize auction
        assert_ok!(VcgAuction::finalize_auction(
            RawOrigin::Signed(1).into(),
            auction_id
        ));

        // Check results: Winner should be agent with bid 100, payment should be 150
        let auction = VcgAuction::auctions(auction_id).unwrap();
        assert_eq!(auction.status, AuctionStatus::Finalized);
        let expected_did: BoundedVec<u8, ConstU32<128>> =
            2u64.to_be_bytes().to_vec().try_into().unwrap();
        assert_eq!(auction.winner.as_ref().unwrap(), &expected_did);
        assert_eq!(auction.payment_amount, Some(150));

        // Check event
        System::assert_last_event(
            Event::AuctionFinalized {
                auction_id,
                winner_did: 2u64.to_be_bytes().to_vec(),
                winning_bid: 100,
                payment_amount: 150,
            }
            .into(),
        );
    });
}

#[test]
fn vcg_auction_two_bidders_scenario() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Setup: Register two agents
        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);
        register_test_agent(3, b"agent2", vec![b"math".to_vec()]);

        let auction_id = create_test_auction();

        // Place bids: [100, 200]
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(2).into(),
            auction_id,
            100
        ));
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(3).into(),
            auction_id,
            200
        ));

        // Fast forward and finalize
        System::set_block_number(101);
        assert_ok!(VcgAuction::finalize_auction(
            RawOrigin::Signed(1).into(),
            auction_id
        ));

        // Check results: Winner pays second-lowest bid (200)
        let auction = VcgAuction::auctions(auction_id).unwrap();
        assert_eq!(auction.payment_amount, Some(200));
    });
}

#[test]
fn vcg_auction_single_bidder_scenario() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Setup: Register one agent
        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);

        let auction_id = create_test_auction();

        // Place single bid: [100]
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(2).into(),
            auction_id,
            100
        ));

        // Fast forward and finalize
        System::set_block_number(101);
        assert_ok!(VcgAuction::finalize_auction(
            RawOrigin::Signed(1).into(),
            auction_id
        ));

        // Check results: Winner pays their own bid (no competition)
        let auction = VcgAuction::auctions(auction_id).unwrap();
        assert_eq!(auction.payment_amount, Some(100));
    });
}

#[test]
fn vcg_auction_tie_scenario() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Setup: Register three agents
        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);
        register_test_agent(3, b"agent2", vec![b"math".to_vec()]);
        register_test_agent(4, b"agent3", vec![b"math".to_vec()]);

        let auction_id = create_test_auction();

        // Place bids with tie: [100, 100, 200]
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(2).into(),
            auction_id,
            100
        ));
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(3).into(),
            auction_id,
            100
        ));
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(4).into(),
            auction_id,
            200
        ));

        // Fast forward and finalize
        System::set_block_number(101);
        assert_ok!(VcgAuction::finalize_auction(
            RawOrigin::Signed(1).into(),
            auction_id
        ));

        // Check results: One of the tied winners, payment should be 100 (second-lowest among tied)
        let auction = VcgAuction::auctions(auction_id).unwrap();
        assert_eq!(auction.status, AuctionStatus::Finalized);
        assert_eq!(auction.payment_amount, Some(100)); // Should pay the other tied bid amount

        // Winner should be one of the agents with 100 bid
        let winner_did = auction.winner.as_ref().unwrap();
        let agent1_did: BoundedVec<u8, ConstU32<128>> =
            2u64.to_be_bytes().to_vec().try_into().unwrap();
        let agent2_did: BoundedVec<u8, ConstU32<128>> =
            3u64.to_be_bytes().to_vec().try_into().unwrap();
        assert!(winner_did == &agent1_did || winner_did == &agent2_did);
    });
}

#[test]
fn finalize_auction_fails_before_end() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        let auction_id = create_test_auction();

        // Try to finalize before auction ends (still at block 0, ends at block 100)
        assert_noop!(
            VcgAuction::finalize_auction(RawOrigin::Signed(1).into(), auction_id),
            Error::<Test>::AuctionNotOpen
        );
    });
}

#[test]
fn finalize_auction_fails_with_no_bids() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        let auction_id = create_test_auction();

        // Fast forward past end
        System::set_block_number(101);

        // Try to finalize without any bids
        assert_noop!(
            VcgAuction::finalize_auction(RawOrigin::Signed(1).into(), auction_id),
            Error::<Test>::NoBidsToFinalize
        );
    });
}

#[test]
fn cancel_auction_works() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        let auction_id = create_test_auction();

        // Cancel auction (no bids placed)
        assert_ok!(VcgAuction::cancel_auction(
            RawOrigin::Signed(1).into(),
            auction_id
        ));

        // Check status
        let auction = VcgAuction::auctions(auction_id).unwrap();
        assert_eq!(auction.status, AuctionStatus::Cancelled);

        // Check event
        System::assert_last_event(Event::AuctionCancelled { auction_id }.into());
    });
}

#[test]
fn cancel_auction_fails_with_bids() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);

        // Setup
        register_test_agent(2, b"agent1", vec![b"math".to_vec()]);
        let auction_id = create_test_auction();

        // Place a bid
        assert_ok!(VcgAuction::place_bid(
            RawOrigin::Signed(2).into(),
            auction_id,
            100
        ));

        // Try to cancel (should fail)
        assert_noop!(
            VcgAuction::cancel_auction(RawOrigin::Signed(1).into(), auction_id),
            Error::<Test>::CannotCancelAuction
        );
    });
}

#[test]
fn strategy_proof_verification() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);
        // Test the strategy-proof property verification
        let mut bids = BoundedVec::default();

        // Create test bids
        let bid1 = Bid {
            agent_did: b"agent1".to_vec().try_into().unwrap(),
            amount: 100,
            placed_at: 1,
        };
        let bid2 = Bid {
            agent_did: b"agent2".to_vec().try_into().unwrap(),
            amount: 150,
            placed_at: 1,
        };
        let bid3 = Bid {
            agent_did: b"agent3".to_vec().try_into().unwrap(),
            amount: 200,
            placed_at: 1,
        };

        bids.try_push(bid1).unwrap();
        bids.try_push(bid2).unwrap();
        bids.try_push(bid3).unwrap();

        // Verify strategy-proof property
        assert!(VcgAuction::verify_strategy_proof(&bids));

        // Test VCG algorithm directly
        let result = VcgAuction::run_vcg_auction(&bids).unwrap();
        assert_eq!(result.winning_bid, 100);
        assert_eq!(result.payment_amount, 150);
        assert_eq!(result.social_welfare, 350); // 150 + 200
    });
}

#[test]
fn get_active_auctions_works() {
    new_test_ext().execute_with(|| {
        // Go past genesis block so events get deposited
        System::set_block_number(1);
        // Create multiple auctions
        let _auction1 = create_test_auction();

        assert_ok!(VcgAuction::create_auction(
            RawOrigin::Signed(1).into(),
            [2u8; 32],
            vec![b"text".to_vec()],
            None,
        ));

        // Get active auctions
        let active_auctions = VcgAuction::get_active_auctions();
        assert_eq!(active_auctions.len(), 2);

        // Fast forward past end
        System::set_block_number(101);

        // Should be no active auctions now
        let active_auctions = VcgAuction::get_active_auctions();
        assert_eq!(active_auctions.len(), 0);
    });
}
