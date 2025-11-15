// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/AINUToken.sol";
import "../src/AINUStaking.sol";
import "../src/AINUGovernance.sol";

contract AINUGovernanceTest is Test {
    AINUToken public token;
    AINUStaking public staking;
    AINUGovernance public governance;

    address public owner = address(this);
    address public proposer = address(0x1);
    address public voter1 = address(0x2);
    address public voter2 = address(0x3);
    address public voter3 = address(0x4);

    uint256 public constant PROPOSAL_AMOUNT = 100_000 * 10 ** 18;
    uint256 public constant STAKE_AMOUNT = 200_000 * 10 ** 18;
    uint256 public constant VOTING_PERIOD = 7 days;

    function setUp() public {
        // Deploy contracts
        token = new AINUToken(owner);
        staking = new AINUStaking(address(token), owner);
        governance = new AINUGovernance(address(staking), owner);

        // Disable burn for testing
        token.toggleBurnOnTransfer(false);

        // Distribute tokens
        token.transfer(proposer, PROPOSAL_AMOUNT * 2);
        token.transfer(voter1, STAKE_AMOUNT);
        token.transfer(voter2, STAKE_AMOUNT);
        token.transfer(voter3, STAKE_AMOUNT);

        // Setup staking for voters
        vm.startPrank(voter1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 365 days); // 2x voting power
        vm.stopPrank();

        vm.startPrank(voter2);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 730 days); // 3x voting power
        vm.stopPrank();

        vm.startPrank(voter3);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 0); // 1x voting power
        vm.stopPrank();

        // Setup proposer stake
        vm.startPrank(proposer);
        token.approve(address(staking), PROPOSAL_AMOUNT);
        staking.stake(PROPOSAL_AMOUNT, 0);
        vm.stopPrank();
    }

    function testCreateProposal() public {
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        assertEq(proposalId, 0); // IDs start at 0
        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.ACTIVE));
    }

    function testCannotProposeBelowThreshold() public {
        address lowStaker = address(0x999);
        uint256 lowAmount = 50_000 * 10 ** 18;

        token.transfer(lowStaker, lowAmount);

        vm.startPrank(lowStaker);
        token.approve(address(staking), lowAmount);
        staking.stake(lowAmount, 0);

        vm.expectRevert("Insufficient stake to propose");
        governance.propose("Test", AINUGovernance.ProposalType.PARAMETER_CHANGE);
        vm.stopPrank();
    }

    function testVotingOnProposal() public {
        // Create proposal
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // Vote for
        vm.prank(voter1);
        governance.castVote(proposalId, true);

        // Vote against
        vm.prank(voter2);
        governance.castVote(proposalId, false);

        // Verify votes recorded
        (,, , , , uint256 forVotes, uint256 againstVotes,) = governance.getProposal(proposalId);
        
        // voter1: 200K * 3.5x (365 days) = 700K
        // voter2: 200K * 2.5x (730 days) = 500K
        // Note: Time weights are calculated based on lock duration
        assertEq(forVotes, 350_000 * 10 ** 18); // voter1's voting power
        assertEq(againstVotes, 500_000 * 10 ** 18); // voter2's voting power
    }

    function testCannotVoteWithoutStake() public {
        address nonStaker = address(0x888);

        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        vm.prank(nonStaker);
        vm.expectRevert("No voting power");
        governance.castVote(proposalId, true);
    }

    function testCannotVoteTwice() public {
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        vm.startPrank(voter1);
        governance.castVote(proposalId, true);

        vm.expectRevert("Already voted");
        governance.castVote(proposalId, true);
        vm.stopPrank();
    }

    function testProposalSucceeds() public {
        // Create PARAMETER_CHANGE proposal (20% quorum)
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // All voters vote FOR (total: 1.2M voting power)
        vm.prank(voter1);
        governance.castVote(proposalId, true);

        vm.prank(voter2);
        governance.castVote(proposalId, true);

        vm.prank(voter3);
        governance.castVote(proposalId, true);

        // Fast forward past voting period
        vm.warp(block.timestamp + VOTING_PERIOD + 1);

        // Proposal should succeed
        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.SUCCEEDED));
    }

    function testProposalDefeated() public {
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // All voters vote AGAINST
        vm.prank(voter1);
        governance.castVote(proposalId, false);

        vm.prank(voter2);
        governance.castVote(proposalId, false);

        vm.prank(voter3);
        governance.castVote(proposalId, false);

        // Fast forward past voting period
        vm.warp(block.timestamp + VOTING_PERIOD + 1);

        // Proposal should be defeated
        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.DEFEATED));
    }

    function testProposalDefeatedByQuorum() public {
        // Create PROTOCOL_UPGRADE proposal (60% quorum - hard to reach)
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Critical Upgrade",
            AINUGovernance.ProposalType.PROTOCOL_UPGRADE
        );

        // Only one voter votes FOR
        vm.prank(voter1);
        governance.castVote(proposalId, true);

        // Fast forward past voting period
        vm.warp(block.timestamp + VOTING_PERIOD + 1);

        // Proposal should be defeated due to low quorum
        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.DEFEATED));
    }

    function testQueueProposal() public {
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // All vote FOR
        vm.prank(voter1);
        governance.castVote(proposalId, true);
        vm.prank(voter2);
        governance.castVote(proposalId, true);
        vm.prank(voter3);
        governance.castVote(proposalId, true);

        // Fast forward past voting period
        vm.warp(block.timestamp + VOTING_PERIOD + 1);

        // Queue the proposal
        governance.queue(proposalId);

        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.QUEUED));
    }

    function testExecuteProposal() public {
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // All vote FOR
        vm.prank(voter1);
        governance.castVote(proposalId, true);
        vm.prank(voter2);
        governance.castVote(proposalId, true);
        vm.prank(voter3);
        governance.castVote(proposalId, true);

        // Fast forward past voting period
        vm.warp(block.timestamp + VOTING_PERIOD + 1);

        // Queue the proposal
        governance.queue(proposalId);

        // Fast forward past timelock
        vm.warp(block.timestamp + 48 hours + 1);

        // Execute
        governance.execute(proposalId);

        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.EXECUTED));
    }

    function testCannotExecuteBeforeTimelock() public {
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // All vote FOR
        vm.prank(voter1);
        governance.castVote(proposalId, true);
        vm.prank(voter2);
        governance.castVote(proposalId, true);
        vm.prank(voter3);
        governance.castVote(proposalId, true);

        vm.warp(block.timestamp + VOTING_PERIOD + 1);
        governance.queue(proposalId);

        // Try to execute immediately
        vm.expectRevert("Timelock not expired");
        governance.execute(proposalId);
    }

    function testCancelProposal() public {
        vm.prank(proposer);
        uint256 proposalId = governance.propose(
            "Test Proposal",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // Owner can cancel
        governance.cancel(proposalId);

        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.CANCELLED));
    }

    function testDifferentQuorumRequirements() public {
        // Test PARAMETER_CHANGE (20% quorum)
        vm.prank(proposer);
        uint256 proposalId1 = governance.propose(
            "Parameter Change",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // Test TREASURY_SPENDING (40% quorum)
        vm.prank(proposer);
        uint256 proposalId2 = governance.propose(
            "Treasury Spending",
            AINUGovernance.ProposalType.TREASURY_SPENDING
        );

        // Test PROTOCOL_UPGRADE (60% quorum)
        vm.prank(proposer);
        uint256 proposalId3 = governance.propose(
            "Protocol Upgrade",
            AINUGovernance.ProposalType.PROTOCOL_UPGRADE
        );

        // Verify all proposals created
        assertEq(uint8(governance.getProposalState(proposalId1)), uint8(AINUGovernance.ProposalState.ACTIVE));
        assertEq(uint8(governance.getProposalState(proposalId2)), uint8(AINUGovernance.ProposalState.ACTIVE));
        assertEq(uint8(governance.getProposalState(proposalId3)), uint8(AINUGovernance.ProposalState.ACTIVE));
    }
}
