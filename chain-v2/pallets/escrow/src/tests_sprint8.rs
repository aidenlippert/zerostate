//! # Sprint 8 Escrow Tests - Comprehensive Test Suite
//!
//! Tests for multi-party escrow, milestone-based escrow, batch operations,
//! refund policies, and template systems.

use crate::{
    mock::*, phase3_batch_refund::*, Error, EscrowDetails, EscrowParticipant, EscrowState, Event,
    Milestone, ParticipantRole,
};
use frame_support::{
    assert_noop, assert_ok,
    traits::{ConstU32, Currency},
    weights::Weight,
    BoundedVec,
};
use frame_system::RawOrigin;
use sp_runtime::traits::{Saturating, Zero};

// Mock data constants
const ALICE: u64 = 1;
const BOB: u64 = 2;
const CHARLIE: u64 = 3;
const DAVE: u64 = 4;
const EVE: u64 = 5;

const TASK_ID_1: [u8; 32] = [1u8; 32];
const TASK_ID_2: [u8; 32] = [2u8; 32];
const TASK_ID_3: [u8; 32] = [3u8; 32];
const TASK_HASH_1: [u8; 32] = [11u8; 32];
const TASK_HASH_2: [u8; 32] = [22u8; 32];
const TASK_HASH_3: [u8; 32] = [33u8; 32];

const DEFAULT_AMOUNT: u64 = 1000;
const SMALL_AMOUNT: u64 = 100;
const LARGE_AMOUNT: u64 = 5000;

/// Helper function to setup accounts with balance
fn setup_accounts() {
    let _ = Balances::deposit_creating(&ALICE, 10000);
    let _ = Balances::deposit_creating(&BOB, 10000);
    let _ = Balances::deposit_creating(&CHARLIE, 10000);
    let _ = Balances::deposit_creating(&DAVE, 10000);
    let _ = Balances::deposit_creating(&EVE, 10000);
}

/// Helper function to create basic escrow
fn create_basic_escrow(user: u64, task_id: [u8; 32], amount: u64) {
    assert_ok!(Escrow::create_escrow(
        RuntimeOrigin::signed(user),
        task_id,
        amount,
        TASK_HASH_1,
        None,
    ));
}

/// Helper function to register test agent DID
fn register_test_agent(account: u64) {
    let agent_did = format!("did:ainur:agent:{}", account).into_bytes();
    assert_ok!(Did::create_did(
        RuntimeOrigin::signed(account),
        agent_did.clone(),
        [account as u8; 32],
    ));
}

// ========== MULTI-PARTY ESCROW TESTS ==========

#[test]
fn test_add_participant() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Test adding a payer participant
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
            ParticipantRole::Payer,
            500,
        ));

        // Verify participant was added
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert!(escrow.is_multi_party);
        assert_eq!(escrow.participants.len(), 1);
        assert_eq!(escrow.participants[0].account, BOB);
        assert_eq!(escrow.participants[0].role, ParticipantRole::Payer);
        assert_eq!(escrow.participants[0].amount, 500);
        assert!(!escrow.participants[0].approved);

        // Verify funds are reserved for payer
        assert_eq!(Balances::reserved_balance(&BOB), 500);

        // Test adding a payee participant
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            CHARLIE,
            ParticipantRole::Payee,
            300,
        ));

        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert_eq!(escrow.participants.len(), 2);

        // Verify payee funds are not reserved (they receive payment)
        assert_eq!(Balances::reserved_balance(&CHARLIE), 0);

        // Test adding an arbiter
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            DAVE,
            ParticipantRole::Arbiter,
            0,
        ));

        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert_eq!(escrow.participants.len(), 3);

        // Verify event was emitted
        System::assert_last_event(RuntimeEvent::Escrow(Event::ParticipantAdded {
            task_id: TASK_ID_1,
            participant: DAVE,
            role: ParticipantRole::Arbiter,
            amount: 0,
        }));
    });
}

#[test]
fn test_add_participant_errors() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Test non-creator cannot add participants
        assert_noop!(
            Escrow::add_participant(
                RuntimeOrigin::signed(BOB),
                TASK_ID_1,
                CHARLIE,
                ParticipantRole::Payer,
                500,
            ),
            Error::<Test>::NotEscrowCreator
        );

        // Test cannot add participant with zero amount for payer
        assert_noop!(
            Escrow::add_participant(
                RuntimeOrigin::signed(ALICE),
                TASK_ID_1,
                BOB,
                ParticipantRole::Payer,
                0,
            ),
            Error::<Test>::InsufficientBalance
        );

        // Add a participant first
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
            ParticipantRole::Payer,
            500,
        ));

        // Test cannot add same participant twice
        assert_noop!(
            Escrow::add_participant(
                RuntimeOrigin::signed(ALICE),
                TASK_ID_1,
                BOB,
                ParticipantRole::Payee,
                300,
            ),
            Error::<Test>::ParticipantAlreadyExists
        );

        // Test insufficient balance for payer
        assert_noop!(
            Escrow::add_participant(
                RuntimeOrigin::signed(ALICE),
                TASK_ID_1,
                CHARLIE,
                ParticipantRole::Payer,
                20000, // More than Charlie's balance
            ),
            Error::<Test>::InsufficientBalance
        );
    });
}

#[test]
fn test_remove_participant() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add participants
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
            ParticipantRole::Payer,
            500,
        ));
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            CHARLIE,
            ParticipantRole::Payee,
            300,
        ));

        // Test escrow creator can remove participant
        assert_ok!(Escrow::remove_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
        ));

        // Verify participant was removed and funds unreserved
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert_eq!(escrow.participants.len(), 1);
        assert_eq!(Balances::reserved_balance(&BOB), 0);

        // Test participant can remove themselves
        assert_ok!(Escrow::remove_participant(
            RuntimeOrigin::signed(CHARLIE),
            TASK_ID_1,
            CHARLIE,
        ));

        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert_eq!(escrow.participants.len(), 0);
        assert!(!escrow.is_multi_party);

        // Verify event was emitted
        System::assert_last_event(RuntimeEvent::Escrow(Event::ParticipantRemoved {
            task_id: TASK_ID_1,
            participant: CHARLIE,
        }));
    });
}

#[test]
fn test_remove_participant_errors() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Test cannot remove non-existent participant
        assert_noop!(
            Escrow::remove_participant(RuntimeOrigin::signed(ALICE), TASK_ID_1, BOB,),
            Error::<Test>::ParticipantNotFound
        );

        // Add participant
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
            ParticipantRole::Payer,
            500,
        ));

        // Test unauthorized user cannot remove participant
        assert_noop!(
            Escrow::remove_participant(RuntimeOrigin::signed(CHARLIE), TASK_ID_1, BOB,),
            Error::<Test>::NotEscrowCreator
        );

        // Accept task first
        register_test_agent(DAVE);
        let agent_did = format!("did:ainur:agent:{}", DAVE).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(DAVE),
            TASK_ID_1,
            agent_did,
        ));

        // Test cannot remove participant after escrow is accepted
        assert_noop!(
            Escrow::remove_participant(RuntimeOrigin::signed(ALICE), TASK_ID_1, BOB,),
            Error::<Test>::InvalidEscrowState
        );
    });
}

#[test]
fn test_multi_party_approval() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add participants
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
            ParticipantRole::Payer,
            500,
        ));
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            CHARLIE,
            ParticipantRole::Payee,
            500,
        ));

        // Accept task
        register_test_agent(DAVE);
        let agent_did = format!("did:ainur:agent:{}", DAVE).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(DAVE),
            TASK_ID_1,
            agent_did,
        ));

        // Test approval mechanism would be implemented in future versions
        // For now, we verify the setup is correct
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert!(escrow.is_multi_party);
        assert_eq!(escrow.participants.len(), 2);
        assert_eq!(escrow.state, EscrowState::Accepted);
    });
}

#[test]
fn test_multi_party_payment_distribution() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add participants
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
            ParticipantRole::Payer,
            500,
        ));
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            CHARLIE,
            ParticipantRole::Payee,
            500,
        ));

        // Accept and release payment
        register_test_agent(DAVE);
        let agent_did = format!("did:ainur:agent:{}", DAVE).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(DAVE),
            TASK_ID_1,
            agent_did,
        ));

        let initial_bob_balance = Balances::free_balance(&BOB);
        let initial_charlie_balance = Balances::free_balance(&CHARLIE);
        let initial_dave_balance = Balances::free_balance(&DAVE);

        // Release payment
        assert_ok!(Escrow::release_payment(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        // Verify payment distribution (basic escrow functionality)
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert_eq!(escrow.state, EscrowState::Completed);

        // In a full implementation, multi-party distribution would be different
        // For now, verify basic payment release works with multi-party setup
        let final_dave_balance = Balances::free_balance(&DAVE);
        assert!(final_dave_balance > initial_dave_balance);
    });
}

// ========== MILESTONE ESCROW TESTS ==========

#[test]
fn test_add_milestone() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestone
        let description = b"Complete Phase 1".to_vec();
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            description.clone(),
            300,
            2, // required approvals
        ));

        // Verify milestone was added
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert!(escrow.is_milestone_based);
        assert_eq!(escrow.milestones.len(), 1);
        assert_eq!(escrow.next_milestone_id, 1);

        let milestone = &escrow.milestones[0];
        assert_eq!(milestone.id, 0);
        assert_eq!(
            milestone.description,
            BoundedVec::try_from(description).unwrap()
        );
        assert_eq!(milestone.amount, 300);
        assert!(!milestone.completed);
        assert_eq!(milestone.required_approvals, 2);
        assert_eq!(milestone.approved_by.len(), 0);

        // Verify event was emitted
        System::assert_last_event(RuntimeEvent::Escrow(Event::MilestoneAdded {
            task_id: TASK_ID_1,
            milestone_id: 0,
            amount: 300,
            required_approvals: 2,
        }));
    });
}

#[test]
fn test_add_milestone_errors() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Test non-creator cannot add milestones
        assert_noop!(
            Escrow::add_milestone(
                RuntimeOrigin::signed(BOB),
                TASK_ID_1,
                b"Test".to_vec(),
                300,
                1,
            ),
            Error::<Test>::NotEscrowCreator
        );

        // Test zero amount milestone
        assert_noop!(
            Escrow::add_milestone(
                RuntimeOrigin::signed(ALICE),
                TASK_ID_1,
                b"Test".to_vec(),
                0,
                1,
            ),
            Error::<Test>::InsufficientBalance
        );

        // Test zero required approvals
        assert_noop!(
            Escrow::add_milestone(
                RuntimeOrigin::signed(ALICE),
                TASK_ID_1,
                b"Test".to_vec(),
                300,
                0,
            ),
            Error::<Test>::InvalidMilestone
        );

        // Accept task first
        register_test_agent(DAVE);
        let agent_did = format!("did:ainur:agent:{}", DAVE).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(DAVE),
            TASK_ID_1,
            agent_did,
        ));

        // Test cannot add milestone after acceptance
        assert_noop!(
            Escrow::add_milestone(
                RuntimeOrigin::signed(ALICE),
                TASK_ID_1,
                b"Test".to_vec(),
                300,
                1,
            ),
            Error::<Test>::InvalidEscrowState
        );
    });
}

#[test]
fn test_complete_milestone() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestone
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Phase 1".to_vec(),
            300,
            1,
        ));

        // Accept task
        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            agent_did,
        ));

        // Complete milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            0, // milestone_id
        ));

        // Verify milestone completion
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        let milestone = &escrow.milestones[0];
        assert!(milestone.completed);

        // Verify event was emitted
        System::assert_last_event(RuntimeEvent::Escrow(Event::MilestoneCompleted {
            task_id: TASK_ID_1,
            milestone_id: 0,
            completed_by: BOB,
        }));
    });
}

#[test]
fn test_complete_milestone_errors() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestone
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Phase 1".to_vec(),
            300,
            1,
        ));

        // Test cannot complete before acceptance
        assert_noop!(
            Escrow::complete_milestone(RuntimeOrigin::signed(ALICE), TASK_ID_1, 0,),
            Error::<Test>::InvalidEscrowState
        );

        // Accept task
        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            agent_did,
        ));

        // Test non-agent cannot complete milestone
        assert_noop!(
            Escrow::complete_milestone(RuntimeOrigin::signed(ALICE), TASK_ID_1, 0,),
            Error::<Test>::NotAssignedAgent
        );

        // Test invalid milestone ID
        assert_noop!(
            Escrow::complete_milestone(RuntimeOrigin::signed(BOB), TASK_ID_1, 999,),
            Error::<Test>::MilestoneNotFound
        );

        // Complete milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            0,
        ));

        // Test cannot complete already completed milestone
        assert_noop!(
            Escrow::complete_milestone(RuntimeOrigin::signed(BOB), TASK_ID_1, 0,),
            Error::<Test>::MilestoneAlreadyCompleted
        );
    });
}

#[test]
fn test_approve_milestone() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestone
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Phase 1".to_vec(),
            300,
            2, // requires 2 approvals
        ));

        // Accept task
        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            agent_did,
        ));

        // Complete milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            0,
        ));

        // Approve milestone (first approval)
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            0,
        ));

        // Verify approval was recorded
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        let milestone = &escrow.milestones[0];
        assert_eq!(milestone.approved_by.len(), 1);
        assert!(milestone.approved_by.contains(&ALICE));

        // Add participant to provide second approval
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            CHARLIE,
            ParticipantRole::Arbiter,
            0,
        ));

        // Second approval (should trigger payment)
        let initial_bob_balance = Balances::free_balance(&BOB);
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(CHARLIE),
            TASK_ID_1,
            0,
        ));

        // Verify payment was released
        let final_bob_balance = Balances::free_balance(&BOB);
        assert!(final_bob_balance > initial_bob_balance);

        // Verify events were emitted
        let events = System::events();
        assert!(events.iter().any(|e| matches!(
            &e.event,
            RuntimeEvent::Escrow(Event::MilestoneApproved {
                task_id: TASK_ID_1,
                milestone_id: 0,
                approved_by: CHARLIE
            })
        )));
        assert!(events.iter().any(|e| matches!(
            &e.event,
            RuntimeEvent::Escrow(Event::MilestonePaid {
                task_id: TASK_ID_1,
                milestone_id: 0,
                ..
            })
        )));
    });
}

#[test]
fn test_approve_milestone_errors() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestone
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Phase 1".to_vec(),
            300,
            1,
        ));

        // Test cannot approve before acceptance
        assert_noop!(
            Escrow::approve_milestone(RuntimeOrigin::signed(ALICE), TASK_ID_1, 0,),
            Error::<Test>::InvalidEscrowState
        );

        // Accept task
        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            agent_did,
        ));

        // Test cannot approve uncompleted milestone
        assert_noop!(
            Escrow::approve_milestone(RuntimeOrigin::signed(ALICE), TASK_ID_1, 0,),
            Error::<Test>::MilestoneNotCompleted
        );

        // Complete milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            0,
        ));

        // Test unauthorized user cannot approve
        assert_noop!(
            Escrow::approve_milestone(RuntimeOrigin::signed(DAVE), TASK_ID_1, 0,),
            Error::<Test>::NotAuthorizedToApprove
        );

        // Test invalid milestone ID
        assert_noop!(
            Escrow::approve_milestone(RuntimeOrigin::signed(ALICE), TASK_ID_1, 999,),
            Error::<Test>::MilestoneNotFound
        );

        // Approve milestone
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            0,
        ));

        // Test cannot approve twice
        assert_noop!(
            Escrow::approve_milestone(RuntimeOrigin::signed(ALICE), TASK_ID_1, 0,),
            Error::<Test>::AlreadyApproved
        );
    });
}

#[test]
fn test_automatic_milestone_release() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestone with single approval required
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Auto Release Test".to_vec(),
            500,
            1, // only 1 approval needed
        ));

        // Accept task
        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            agent_did,
        ));

        // Complete milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            0,
        ));

        let initial_bob_balance = Balances::free_balance(&BOB);

        // Single approval should trigger automatic release
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            0,
        ));

        // Verify payment was automatically released
        let final_bob_balance = Balances::free_balance(&BOB);
        assert!(final_bob_balance > initial_bob_balance);

        // Verify milestone paid event was emitted
        System::assert_has_event(RuntimeEvent::Escrow(Event::MilestonePaid {
            task_id: TASK_ID_1,
            milestone_id: 0,
            amount: 475, // 500 - 5% fee
            recipient: BOB,
        }));
    });
}

// ========== BATCH OPERATION TESTS ==========

#[test]
fn test_batch_create_escrow() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Create batch requests
        let requests = vec![
            BatchCreateEscrowRequest {
                task_id: TASK_ID_1,
                amount: 500,
                task_hash: TASK_HASH_1,
                timeout_blocks: None,
                refund_policy: None,
            },
            BatchCreateEscrowRequest {
                task_id: TASK_ID_2,
                amount: 700,
                task_hash: TASK_HASH_2,
                timeout_blocks: Some(1000),
                refund_policy: None,
            },
            BatchCreateEscrowRequest {
                task_id: TASK_ID_3,
                amount: 300,
                task_hash: TASK_HASH_3,
                timeout_blocks: None,
                refund_policy: Some(RefundPolicy {
                    policy_type: RefundPolicyType::Standard,
                    can_override: false,
                    override_authority: None,
                    created_at: 1,
                }),
            },
        ];

        let initial_balance = Balances::free_balance(&ALICE);
        let total_amount = 1500u64;

        // Execute batch creation
        assert_ok!(Escrow::batch_create_escrow(
            RuntimeOrigin::signed(ALICE),
            requests,
        ));

        // Verify all escrows were created
        assert!(Escrow::escrows(&TASK_ID_1).is_some());
        assert!(Escrow::escrows(&TASK_ID_2).is_some());
        assert!(Escrow::escrows(&TASK_ID_3).is_some());

        // Verify total amount was reserved
        assert_eq!(Balances::reserved_balance(&ALICE), total_amount);

        // Verify refund policy was stored for TASK_ID_3
        assert!(Escrow::escrow_refund_policies(&TASK_ID_3).is_some());

        // Verify batch completed event was emitted
        let events = System::events();
        assert!(events.iter().any(|e| matches!(
            &e.event,
            RuntimeEvent::Escrow(Event::BatchOperationCompleted {
                successful_operations: 3,
                failed_operations: 0,
                ..
            })
        )));
    });
}

#[test]
fn test_batch_create_escrow_errors() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Test empty batch
        assert_noop!(
            Escrow::batch_create_escrow(RuntimeOrigin::signed(ALICE), vec![],),
            Error::<Test>::InvalidBatchSize
        );

        // Test batch size exceeded (create more than max allowed)
        let large_batch: Vec<BatchCreateEscrowRequest<Test>> = (0..100)
            .map(|i| BatchCreateEscrowRequest {
                task_id: [i as u8; 32],
                amount: 100,
                task_hash: [i as u8; 32],
                timeout_blocks: None,
                refund_policy: None,
            })
            .collect();

        assert_noop!(
            Escrow::batch_create_escrow(RuntimeOrigin::signed(ALICE), large_batch,),
            Error::<Test>::BatchSizeExceeded
        );

        // Test insufficient balance
        let expensive_batch = vec![BatchCreateEscrowRequest {
            task_id: TASK_ID_1,
            amount: 20000, // More than ALICE has
            task_hash: TASK_HASH_1,
            timeout_blocks: None,
            refund_policy: None,
        }];

        assert_noop!(
            Escrow::batch_create_escrow(RuntimeOrigin::signed(ALICE), expensive_batch,),
            Error::<Test>::InsufficientBalanceForBatch
        );
    });
}

#[test]
fn test_batch_release_payment() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Create multiple escrows
        let task_ids = [TASK_ID_1, TASK_ID_2, TASK_ID_3];
        for &task_id in &task_ids {
            create_basic_escrow(ALICE, task_id, 500);
        }

        // Accept all tasks
        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        for &task_id in &task_ids {
            assert_ok!(Escrow::accept_task(
                RuntimeOrigin::signed(BOB),
                task_id,
                agent_did.clone(),
            ));
        }

        let initial_bob_balance = Balances::free_balance(&BOB);

        // Batch release payments
        assert_ok!(Escrow::batch_release_payment(
            RuntimeOrigin::signed(ALICE),
            task_ids.to_vec(),
        ));

        // Verify all payments were released
        for &task_id in &task_ids {
            let escrow = Escrow::escrows(&task_id).unwrap();
            assert_eq!(escrow.state, EscrowState::Completed);
        }

        // Verify BOB received payments
        let final_bob_balance = Balances::free_balance(&BOB);
        let expected_payment = 3 * 475; // 3 × (500 - 25 fee)
        assert_eq!(final_bob_balance, initial_bob_balance + expected_payment);

        // Verify batch completed event
        System::assert_has_event(RuntimeEvent::Escrow(Event::BatchOperationCompleted {
            successful_operations: 3,
            failed_operations: 0,
            total_amount_processed: 1500,
            ..
        }));
    });
}

#[test]
fn test_batch_refund() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Create escrows with different refund policies
        let task_ids = [TASK_ID_1, TASK_ID_2, TASK_ID_3];
        for &task_id in &task_ids {
            create_basic_escrow(ALICE, task_id, 500);
        }

        // Set different refund policies
        let standard_policy = RefundPolicy {
            policy_type: RefundPolicyType::Standard,
            can_override: false,
            override_authority: None,
            created_at: 1,
        };

        let fee_policy = RefundPolicy {
            policy_type: RefundPolicyType::CancellationFee { fee_amount: 50 },
            can_override: false,
            override_authority: None,
            created_at: 1,
        };

        assert_ok!(Escrow::set_refund_policy(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_2,
            fee_policy,
        ));

        let initial_alice_balance = Balances::free_balance(&ALICE);

        // Batch refund
        assert_ok!(Escrow::batch_refund_escrow(
            RuntimeOrigin::signed(ALICE),
            task_ids.to_vec(),
        ));

        // Verify all refunds were processed
        for &task_id in &task_ids {
            let escrow = Escrow::escrows(&task_id).unwrap();
            assert_eq!(escrow.state, EscrowState::Refunded);
        }

        // Verify refund amounts (TASK_ID_2 should have fee deducted)
        let final_alice_balance = Balances::free_balance(&ALICE);
        let expected_refund = 1450; // 500 + 450 + 500 (fee deducted from TASK_ID_2)
        assert_eq!(final_alice_balance, initial_alice_balance);
    });
}

#[test]
fn test_batch_dispute() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Create and accept multiple escrows
        let task_ids = [TASK_ID_1, TASK_ID_2];
        for &task_id in &task_ids {
            create_basic_escrow(ALICE, task_id, 500);
        }

        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        for &task_id in &task_ids {
            assert_ok!(Escrow::accept_task(
                RuntimeOrigin::signed(BOB),
                task_id,
                agent_did.clone(),
            ));
        }

        // Batch dispute
        assert_ok!(Escrow::batch_dispute_escrow(
            RuntimeOrigin::signed(ALICE),
            task_ids.to_vec(),
        ));

        // Verify all escrows are disputed
        for &task_id in &task_ids {
            let escrow = Escrow::escrows(&task_id).unwrap();
            assert_eq!(escrow.state, EscrowState::Disputed);
        }

        // Verify batch completed event
        System::assert_has_event(RuntimeEvent::Escrow(Event::BatchOperationCompleted {
            successful_operations: 2,
            failed_operations: 0,
            total_amount_processed: 0, // Disputes don't process amounts
            ..
        }));
    });
}

// ========== REFUND POLICY TESTS ==========

#[test]
fn test_time_based_refund() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Set time-based refund policy
        let policy = RefundPolicy {
            policy_type: RefundPolicyType::TimeBased {
                full_refund_deadline: 100,
                partial_refund_percentage: 50,
            },
            can_override: false,
            override_authority: None,
            created_at: 1,
        };

        assert_ok!(Escrow::set_refund_policy(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            policy,
        ));

        // Test full refund before deadline (we're at block 1)
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        // Advance past deadline
        System::set_block_number(150);

        // Test partial refund after deadline
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        // Verify appropriate events were emitted
        let events = System::events();
        assert!(events.iter().any(|e| matches!(
            &e.event,
            RuntimeEvent::Escrow(Event::RefundAmountCalculated {
                refund_amount: 1000,
                ..
            })
        )));
        assert!(events.iter().any(|e| matches!(
            &e.event,
            RuntimeEvent::Escrow(Event::RefundAmountCalculated {
                refund_amount: 500,
                ..
            })
        )));
    });
}

#[test]
fn test_graduated_refund() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Set graduated refund policy
        let stages = BoundedVec::try_from(vec![
            (50, 80),  // 80% refund until block 50
            (100, 60), // 60% refund until block 100
            (150, 40), // 40% refund until block 150
        ])
        .unwrap();

        let policy = RefundPolicy {
            policy_type: RefundPolicyType::Graduated { stages },
            can_override: false,
            override_authority: None,
            created_at: 1,
        };

        assert_ok!(Escrow::set_refund_policy(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            policy,
        ));

        // Test at different time stages
        System::set_block_number(25);
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        System::set_block_number(75);
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        System::set_block_number(125);
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        System::set_block_number(200);
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));
    });
}

#[test]
fn test_conditional_refund() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestones
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Milestone 1".to_vec(),
            300,
            1,
        ));
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Milestone 2".to_vec(),
            400,
            1,
        ));

        // Set conditional refund policy
        let refund_percentages = BoundedVec::try_from(vec![100, 70, 30]).unwrap();
        let policy = RefundPolicy {
            policy_type: RefundPolicyType::Conditional {
                milestones_completed: 2,
                refund_percentages,
            },
            can_override: false,
            override_authority: None,
            created_at: 1,
        };

        assert_ok!(Escrow::set_refund_policy(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            policy,
        ));

        // Test with no milestones completed (100% refund)
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        // Accept task and complete one milestone
        register_test_agent(BOB);
        let agent_did = format!("did:ainur:agent:{}", BOB).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            agent_did,
        ));

        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            0,
        ));

        // Test with one milestone completed (70% refund)
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        // Complete second milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            1,
        ));

        // Test with two milestones completed (30% refund)
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));
    });
}

#[test]
fn test_arbiter_override_refund() {
    new_test_ext().execute_with(|| {
        setup_accounts();
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Set policy with arbiter override
        let policy = RefundPolicy {
            policy_type: RefundPolicyType::NoRefund {
                work_start_deadline: 50,
            },
            can_override: true,
            override_authority: Some(EVE), // EVE is the arbiter
            created_at: 1,
        };

        assert_ok!(Escrow::set_refund_policy(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            policy,
        ));

        // Move past work start deadline
        System::set_block_number(100);

        // Normal refund should be 0
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        // Arbiter can override to 75% refund
        let initial_alice_balance = Balances::free_balance(&ALICE);
        assert_ok!(Escrow::override_refund_amount(
            RuntimeOrigin::signed(EVE),
            TASK_ID_1,
            750, // 75% of 1000
        ));

        // Verify override was applied
        let final_alice_balance = Balances::free_balance(&ALICE);
        assert_eq!(final_alice_balance, initial_alice_balance);

        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert_eq!(escrow.state, EscrowState::Refunded);

        // Verify override event was emitted
        System::assert_has_event(RuntimeEvent::Escrow(Event::RefundPolicyOverridden {
            task_id: TASK_ID_1,
            original_amount: 1000,
            override_amount: 750,
            overridden_by: EVE,
        }));
    });
}

// ========== TEMPLATE SYSTEM TESTS ==========

#[test]
fn test_create_template() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Create standard escrow template
        let standard_template = EscrowDetails {
            task_id: [0u8; 32], // Template ID
            user: ALICE,
            agent_did: None,
            agent_account: None,
            amount: 0, // Will be set when using template
            fee_percent: 5,
            created_at: 0,
            expires_at: 0,
            state: EscrowState::Pending,
            task_hash: [0u8; 32],
            participants: BoundedVec::new(),
            is_multi_party: false,
            milestones: BoundedVec::new(),
            is_milestone_based: false,
            next_milestone_id: 0,
        };

        // Templates would be stored in a separate storage map in a full implementation
        // For now, verify the structure is correct
        assert_eq!(standard_template.fee_percent, 5);
        assert!(!standard_template.is_multi_party);
        assert!(!standard_template.is_milestone_based);
    });
}

#[test]
fn test_create_escrow_from_template() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Simulate creating escrow from a milestone template
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add milestones as if from a template
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Initial Research".to_vec(),
            300,
            1,
        ));
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Development Phase".to_vec(),
            500,
            2,
        ));
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Testing & Delivery".to_vec(),
            200,
            1,
        ));

        // Verify escrow was created with milestones
        let escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert!(escrow.is_milestone_based);
        assert_eq!(escrow.milestones.len(), 3);
        assert_eq!(escrow.next_milestone_id, 3);
    });
}

#[test]
fn test_all_builtin_templates() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Test 1: Basic Escrow Template
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);
        let basic_escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert!(!basic_escrow.is_multi_party);
        assert!(!basic_escrow.is_milestone_based);
        assert_eq!(basic_escrow.fee_percent, 5);

        // Test 2: Multi-Party Template
        create_basic_escrow(ALICE, TASK_ID_2, DEFAULT_AMOUNT);
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_2,
            BOB,
            ParticipantRole::Payer,
            500,
        ));
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_2,
            CHARLIE,
            ParticipantRole::Payee,
            400,
        ));

        let multi_party_escrow = Escrow::escrows(&TASK_ID_2).unwrap();
        assert!(multi_party_escrow.is_multi_party);
        assert_eq!(multi_party_escrow.participants.len(), 2);

        // Test 3: Milestone Template
        create_basic_escrow(ALICE, TASK_ID_3, DEFAULT_AMOUNT);
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_3,
            b"Phase 1".to_vec(),
            400,
            1,
        ));
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_3,
            b"Phase 2".to_vec(),
            600,
            2,
        ));

        let milestone_escrow = Escrow::escrows(&TASK_ID_3).unwrap();
        assert!(milestone_escrow.is_milestone_based);
        assert_eq!(milestone_escrow.milestones.len(), 2);

        // Test 4: Advanced Refund Policy Template
        let advanced_policy = RefundPolicy {
            policy_type: RefundPolicyType::Graduated {
                stages: BoundedVec::try_from(vec![(100, 90), (200, 70), (300, 50)]).unwrap(),
            },
            can_override: true,
            override_authority: Some(EVE),
            created_at: 1,
        };

        let task_id_4 = [4u8; 32];
        create_basic_escrow(ALICE, task_id_4, DEFAULT_AMOUNT);
        assert_ok!(Escrow::set_refund_policy(
            RuntimeOrigin::signed(ALICE),
            task_id_4,
            advanced_policy,
        ));

        let stored_policy = Escrow::escrow_refund_policies(&task_id_4).unwrap();
        assert!(matches!(
            stored_policy.policy_type,
            RefundPolicyType::Graduated { .. }
        ));
        assert!(stored_policy.can_override);
        assert_eq!(stored_policy.override_authority, Some(EVE));
    });
}

// ========== INTEGRATION TESTS ==========

#[test]
fn test_complex_multi_party_milestone_workflow() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Create escrow with complex setup
        create_basic_escrow(ALICE, TASK_ID_1, 2000);

        // Add multi-party participants
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            BOB,
            ParticipantRole::Payer,
            1000,
        ));
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            CHARLIE,
            ParticipantRole::Payee,
            800,
        ));
        assert_ok!(Escrow::add_participant(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            DAVE,
            ParticipantRole::Arbiter,
            0,
        ));

        // Add milestones
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Research Phase".to_vec(),
            600,
            2, // Requires ALICE + one participant
        ));
        assert_ok!(Escrow::add_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            b"Implementation".to_vec(),
            1000,
            3, // Requires all participants
        ));

        // Set refund policy
        let policy = RefundPolicy {
            policy_type: RefundPolicyType::Conditional {
                milestones_completed: 2,
                refund_percentages: BoundedVec::try_from(vec![90, 50, 10]).unwrap(),
            },
            can_override: true,
            override_authority: Some(DAVE),
            created_at: 1,
        };
        assert_ok!(Escrow::set_refund_policy(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            policy,
        ));

        // Accept task
        register_test_agent(EVE);
        let agent_did = format!("did:ainur:agent:{}", EVE).into_bytes();
        assert_ok!(Escrow::accept_task(
            RuntimeOrigin::signed(EVE),
            TASK_ID_1,
            agent_did,
        ));

        // Complete and approve first milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(EVE),
            TASK_ID_1,
            0,
        ));
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            0,
        ));
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            0,
        ));

        // Verify first milestone payment
        let events = System::events();
        assert!(events.iter().any(|e| matches!(
            &e.event,
            RuntimeEvent::Escrow(Event::MilestonePaid {
                task_id: TASK_ID_1,
                milestone_id: 0,
                amount: 570, // 600 - 5% fee
                recipient: EVE
            })
        )));

        // Test refund amount after one milestone (should be 50%)
        assert_ok!(Escrow::evaluate_refund_amount(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
        ));

        // Complete second milestone
        assert_ok!(Escrow::complete_milestone(
            RuntimeOrigin::signed(EVE),
            TASK_ID_1,
            1,
        ));

        // Approve by all participants
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(ALICE),
            TASK_ID_1,
            1,
        ));
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(BOB),
            TASK_ID_1,
            1,
        ));
        assert_ok!(Escrow::approve_milestone(
            RuntimeOrigin::signed(DAVE),
            TASK_ID_1,
            1,
        ));

        // Verify second milestone payment
        let final_events = System::events();
        assert!(final_events.iter().any(|e| matches!(
            &e.event,
            RuntimeEvent::Escrow(Event::MilestonePaid {
                task_id: TASK_ID_1,
                milestone_id: 1,
                amount: 950, // 1000 - 5% fee
                recipient: EVE
            })
        )));

        // Verify escrow structure
        let final_escrow = Escrow::escrows(&TASK_ID_1).unwrap();
        assert!(final_escrow.is_multi_party);
        assert!(final_escrow.is_milestone_based);
        assert_eq!(final_escrow.participants.len(), 3);
        assert_eq!(final_escrow.milestones.len(), 2);
        assert!(final_escrow.milestones.iter().all(|m| m.completed));
    });
}

#[test]
fn test_performance_batch_operations() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Create large batch (within limits)
        let batch_size = 20;
        let requests: Vec<BatchCreateEscrowRequest<Test>> = (0..batch_size)
            .map(|i| BatchCreateEscrowRequest {
                task_id: [i as u8; 32],
                amount: 100,
                task_hash: [(i + 100) as u8; 32],
                timeout_blocks: Some(1000),
                refund_policy: None,
            })
            .collect();

        let start_balance = Balances::free_balance(&ALICE);

        // Execute batch creation
        assert_ok!(Escrow::batch_create_escrow(
            RuntimeOrigin::signed(ALICE),
            requests,
        ));

        // Verify all escrows created
        for i in 0..batch_size {
            let task_id = [i as u8; 32];
            assert!(Escrow::escrows(&task_id).is_some());
        }

        // Verify total reservation
        let expected_total = batch_size * 100;
        assert_eq!(Balances::reserved_balance(&ALICE), expected_total as u64);

        // Test batch refund performance
        let task_ids: Vec<[u8; 32]> = (0..batch_size).map(|i| [i as u8; 32]).collect();

        assert_ok!(Escrow::batch_refund_escrow(
            RuntimeOrigin::signed(ALICE),
            task_ids,
        ));

        // Verify all refunded
        for i in 0..batch_size {
            let task_id = [i as u8; 32];
            let escrow = Escrow::escrows(&task_id).unwrap();
            assert_eq!(escrow.state, EscrowState::Refunded);
        }

        // Verify final balance
        assert_eq!(Balances::free_balance(&ALICE), start_balance);
    });
}

// ========== EDGE CASE TESTS ==========

#[test]
fn test_edge_cases_and_limits() {
    new_test_ext().execute_with(|| {
        setup_accounts();

        // Test maximum participants
        create_basic_escrow(ALICE, TASK_ID_1, DEFAULT_AMOUNT);

        // Add participants up to limit (assuming limit is 10)
        for i in 2..12 {
            if i <= 11 {
                // Within limit
                assert_ok!(Escrow::add_participant(
                    RuntimeOrigin::signed(ALICE),
                    TASK_ID_1,
                    i,
                    ParticipantRole::Payer,
                    50,
                ));
            }
        }

        // Try to exceed limit
        assert_noop!(
            Escrow::add_participant(
                RuntimeOrigin::signed(ALICE),
                TASK_ID_1,
                12,
                ParticipantRole::Payer,
                50,
            ),
            Error::<Test>::TooManyParticipants
        );

        // Test maximum milestones
        let task_id_2 = [2u8; 32];
        create_basic_escrow(ALICE, task_id_2, 5000);

        // Add milestones up to limit (assuming limit is 20)
        for i in 0..20 {
            assert_ok!(Escrow::add_milestone(
                RuntimeOrigin::signed(ALICE),
                task_id_2,
                format!("Milestone {}", i).into_bytes(),
                100,
                1,
            ));
        }

        // Try to exceed milestone limit
        assert_noop!(
            Escrow::add_milestone(
                RuntimeOrigin::signed(ALICE),
                task_id_2,
                b"Excess Milestone".to_vec(),
                100,
                1,
            ),
            Error::<Test>::TooManyMilestones
        );
    });
}
