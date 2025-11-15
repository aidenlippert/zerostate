// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "./AINUStaking.sol";

/**
 * @title AINU Governance
 * @notice On-chain governance for the Ainur protocol
 * @dev Implements:
 *   - Proposal creation (requires 100K AINU staked)
 *   - Voting with time-weighted power
 *   - Timelocked execution (48 hours)
 *   - Quadratic voting (optional)
 */
contract AINUGovernance is Ownable, ReentrancyGuard {
    /// @notice Staking contract for voting power
    AINUStaking public immutable staking;

    /// @notice Proposal types
    enum ProposalType {
        PROTOCOL_UPGRADE,   // Critical: 60% quorum, 75% approval
        TREASURY_SPENDING,  // Standard: 40% quorum, 66% approval
        PARAMETER_CHANGE    // Minor: 20% quorum, 50% approval
    }

    /// @notice Proposal states
    enum ProposalState {
        PENDING,
        ACTIVE,
        SUCCEEDED,
        DEFEATED,
        QUEUED,
        EXECUTED,
        CANCELLED
    }

    /// @notice Proposal struct
    struct Proposal {
        uint256 id;
        address proposer;
        string description;
        ProposalType proposalType;
        uint256 startTime;
        uint256 endTime;
        uint256 executionTime;
        uint256 forVotes;
        uint256 againstVotes;
        uint256 abstainVotes;
        bool executed;
        bool cancelled;
        mapping(address => bool) hasVoted;
        mapping(address => uint256) votes;
    }

    /// @notice Proposal ID => Proposal
    mapping(uint256 => Proposal) public proposals;

    /// @notice Next proposal ID
    uint256 public nextProposalId;

    /// @notice Minimum stake to create proposal: 100K AINU
    uint256 public constant PROPOSAL_THRESHOLD = 100_000 * 10 ** 18;

    /// @notice Voting period: 7 days
    uint256 public constant VOTING_PERIOD = 7 days;

    /// @notice Timelock period: 48 hours
    uint256 public constant TIMELOCK_PERIOD = 48 hours;

    /// @notice Quorum requirements (in basis points)
    uint256 public constant QUORUM_CRITICAL = 6_000; // 60%
    uint256 public constant QUORUM_STANDARD = 4_000; // 40%
    uint256 public constant QUORUM_MINOR = 2_000; // 20%

    /// @notice Approval requirements (in basis points)
    uint256 public constant APPROVAL_CRITICAL = 7_500; // 75%
    uint256 public constant APPROVAL_STANDARD = 6_600; // 66%
    uint256 public constant APPROVAL_MINOR = 5_000; // 50%

    uint256 public constant BASIS_POINTS = 10_000;

    event ProposalCreated(
        uint256 indexed proposalId,
        address indexed proposer,
        ProposalType proposalType,
        string description
    );
    event VoteCast(uint256 indexed proposalId, address indexed voter, uint256 votes, bool support);
    event ProposalQueued(uint256 indexed proposalId, uint256 executionTime);
    event ProposalExecuted(uint256 indexed proposalId);
    event ProposalCancelled(uint256 indexed proposalId);

    constructor(address _staking, address initialOwner) Ownable(initialOwner) {
        staking = AINUStaking(_staking);
    }

    /**
     * @notice Create a new proposal
     * @param description Proposal description
     * @param proposalType Type of proposal
     * @return proposalId New proposal ID
     */
    function propose(string calldata description, ProposalType proposalType) external returns (uint256 proposalId) {
        // Check proposer has enough stake
        AINUStaking.Stake memory stake = staking.getStake(msg.sender);
        require(stake.amount >= PROPOSAL_THRESHOLD, "Insufficient stake to propose");

        proposalId = nextProposalId++;
        Proposal storage proposal = proposals[proposalId];
        
        proposal.id = proposalId;
        proposal.proposer = msg.sender;
        proposal.description = description;
        proposal.proposalType = proposalType;
        proposal.startTime = block.timestamp;
        proposal.endTime = block.timestamp + VOTING_PERIOD;
        proposal.executionTime = 0;
        proposal.forVotes = 0;
        proposal.againstVotes = 0;
        proposal.abstainVotes = 0;
        proposal.executed = false;
        proposal.cancelled = false;

        emit ProposalCreated(proposalId, msg.sender, proposalType, description);
    }

    /**
     * @notice Cast a vote on a proposal
     * @param proposalId Proposal to vote on
     * @param support True for yes, false for no
     */
    function castVote(uint256 proposalId, bool support) external nonReentrant {
        Proposal storage proposal = proposals[proposalId];
        require(getProposalState(proposalId) == ProposalState.ACTIVE, "Voting not active");
        require(!proposal.hasVoted[msg.sender], "Already voted");

        uint256 votingPower = staking.getVotingPower(msg.sender);
        require(votingPower > 0, "No voting power");

        proposal.hasVoted[msg.sender] = true;
        proposal.votes[msg.sender] = votingPower;

        if (support) {
            proposal.forVotes += votingPower;
        } else {
            proposal.againstVotes += votingPower;
        }

        emit VoteCast(proposalId, msg.sender, votingPower, support);
    }

    /**
     * @notice Queue a succeeded proposal for execution
     * @param proposalId Proposal to queue
     */
    function queue(uint256 proposalId) external {
        require(getProposalState(proposalId) == ProposalState.SUCCEEDED, "Proposal not succeeded");
        
        Proposal storage proposal = proposals[proposalId];
        proposal.executionTime = block.timestamp + TIMELOCK_PERIOD;

        emit ProposalQueued(proposalId, proposal.executionTime);
    }

    /**
     * @notice Execute a queued proposal
     * @param proposalId Proposal to execute
     */
    function execute(uint256 proposalId) external nonReentrant {
        require(getProposalState(proposalId) == ProposalState.QUEUED, "Proposal not queued");
        
        Proposal storage proposal = proposals[proposalId];
        require(block.timestamp >= proposal.executionTime, "Timelock not expired");

        proposal.executed = true;

        // TODO: Add actual execution logic here
        // This would call external contracts or update protocol parameters

        emit ProposalExecuted(proposalId);
    }

    /**
     * @notice Cancel a proposal (only proposer or owner)
     * @param proposalId Proposal to cancel
     */
    function cancel(uint256 proposalId) external {
        Proposal storage proposal = proposals[proposalId];
        require(
            msg.sender == proposal.proposer || msg.sender == owner(),
            "Not authorized to cancel"
        );
        require(!proposal.executed, "Proposal already executed");

        proposal.cancelled = true;

        emit ProposalCancelled(proposalId);
    }

    /**
     * @notice Get the current state of a proposal
     * @param proposalId Proposal ID
     * @return Current state
     */
    function getProposalState(uint256 proposalId) public view returns (ProposalState) {
        Proposal storage proposal = proposals[proposalId];
        
        if (proposal.cancelled) {
            return ProposalState.CANCELLED;
        }

        if (proposal.executed) {
            return ProposalState.EXECUTED;
        }

        if (block.timestamp < proposal.endTime) {
            return ProposalState.ACTIVE;
        }

        // Check if proposal succeeded
        (uint256 quorum, uint256 approval) = _getRequirements(proposal.proposalType);
        uint256 totalVotes = proposal.forVotes + proposal.againstVotes;
        uint256 totalStaked = staking.totalStaked();

        bool quorumReached = (totalVotes * BASIS_POINTS) / totalStaked >= quorum;
        bool approvalReached = totalVotes > 0 && (proposal.forVotes * BASIS_POINTS) / totalVotes >= approval;

        if (!quorumReached || !approvalReached) {
            return ProposalState.DEFEATED;
        }

        if (proposal.executionTime == 0) {
            return ProposalState.SUCCEEDED;
        }

        if (block.timestamp < proposal.executionTime) {
            return ProposalState.QUEUED;
        }

        return ProposalState.QUEUED;
    }

    /**
     * @notice Get proposal details
     * @param proposalId Proposal ID
     */
    function getProposal(uint256 proposalId)
        external
        view
        returns (
            address proposer,
            string memory description,
            ProposalType proposalType,
            uint256 startTime,
            uint256 endTime,
            uint256 forVotes,
            uint256 againstVotes,
            ProposalState state
        )
    {
        Proposal storage proposal = proposals[proposalId];
        return (
            proposal.proposer,
            proposal.description,
            proposal.proposalType,
            proposal.startTime,
            proposal.endTime,
            proposal.forVotes,
            proposal.againstVotes,
            getProposalState(proposalId)
        );
    }

    /**
     * @notice Check if an address has voted on a proposal
     * @param proposalId Proposal ID
     * @param voter Address to check
     * @return Whether they voted
     */
    function hasVoted(uint256 proposalId, address voter) external view returns (bool) {
        return proposals[proposalId].hasVoted[voter];
    }

    /**
     * @notice Get quorum and approval requirements for proposal type
     * @param proposalType Type of proposal
     * @return quorum Quorum requirement (basis points)
     * @return approval Approval requirement (basis points)
     */
    function _getRequirements(ProposalType proposalType)
        internal
        pure
        returns (uint256 quorum, uint256 approval)
    {
        if (proposalType == ProposalType.PROTOCOL_UPGRADE) {
            return (QUORUM_CRITICAL, APPROVAL_CRITICAL);
        } else if (proposalType == ProposalType.TREASURY_SPENDING) {
            return (QUORUM_STANDARD, APPROVAL_STANDARD);
        } else {
            return (QUORUM_MINOR, APPROVAL_MINOR);
        }
    }
}
