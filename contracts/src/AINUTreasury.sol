// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title AINU Treasury
 * @notice Manages protocol revenue and token distribution
 * @dev Implements:
 *   - Revenue collection from task fees
 *   - Token distribution to agents and node operators
 *   - Grant management
 *   - Buyback and burn mechanism
 */
contract AINUTreasury is Ownable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    /// @notice AINU token contract
    IERC20 public immutable ainuToken;

    /// @notice Revenue split percentages (in basis points)
    uint256 public constant AGENT_SHARE = 7_000; // 70%
    uint256 public constant NODE_SHARE = 2_000; // 20%
    uint256 public constant PROTOCOL_SHARE = 500; // 5%
    uint256 public constant BURN_SHARE = 500; // 5%
    uint256 public constant BASIS_POINTS = 10_000;

    /// @notice Total revenue collected
    uint256 public totalRevenue;

    /// @notice Total distributed to agents
    uint256 public totalAgentRewards;

    /// @notice Total distributed to node operators
    uint256 public totalNodeRewards;

    /// @notice Total burned
    uint256 public totalBurned;

    /// @notice Pending rewards for agents
    mapping(address => uint256) public pendingAgentRewards;

    /// @notice Pending rewards for node operators
    mapping(address => uint256) public pendingNodeRewards;

    /// @notice Authorized revenue collectors (e.g., task execution contracts)
    mapping(address => bool) public authorizedCollectors;

    /// @notice Grant proposals
    struct Grant {
        address recipient;
        uint256 amount;
        string description;
        bool executed;
        uint256 timestamp;
    }

    /// @notice Grant ID => Grant details
    mapping(uint256 => Grant) public grants;

    /// @notice Next grant ID
    uint256 public nextGrantId;

    event RevenueCollected(uint256 amount, address indexed from);
    event AgentRewardClaimed(address indexed agent, uint256 amount);
    event NodeRewardClaimed(address indexed node, uint256 amount);
    event TokensBurned(uint256 amount);
    event GrantCreated(uint256 indexed grantId, address indexed recipient, uint256 amount);
    event GrantExecuted(uint256 indexed grantId);
    event CollectorAuthorized(address indexed collector, bool authorized);

    constructor(address _ainuToken, address initialOwner) Ownable(initialOwner) {
        ainuToken = IERC20(_ainuToken);
    }

    /**
     * @notice Collect task fees and split according to protocol rules
     * @param amount Total fee amount
     * @param agent Agent that executed the task
     * @param node Node operator that hosted execution
     */
    function collectTaskFee(uint256 amount, address agent, address node) external nonReentrant {
        require(authorizedCollectors[msg.sender], "Not authorized collector");
        require(agent != address(0) && node != address(0), "Invalid addresses");

        // Transfer tokens to treasury
        ainuToken.safeTransferFrom(msg.sender, address(this), amount);
        totalRevenue += amount;

        // Calculate splits
        uint256 agentAmount = (amount * AGENT_SHARE) / BASIS_POINTS;
        uint256 nodeAmount = (amount * NODE_SHARE) / BASIS_POINTS;
        uint256 burnAmount = (amount * BURN_SHARE) / BASIS_POINTS;
        // Protocol share stays in treasury

        // Update pending rewards
        pendingAgentRewards[agent] += agentAmount;
        pendingNodeRewards[node] += nodeAmount;
        totalAgentRewards += agentAmount;
        totalNodeRewards += nodeAmount;

        // Burn tokens
        if (burnAmount > 0) {
            ainuToken.safeTransfer(address(0xdead), burnAmount);
            totalBurned += burnAmount;
            emit TokensBurned(burnAmount);
        }

        emit RevenueCollected(amount, msg.sender);
    }

    /**
     * @notice Claim pending agent rewards
     */
    function claimAgentRewards() external nonReentrant {
        uint256 pending = pendingAgentRewards[msg.sender];
        require(pending > 0, "No pending rewards");

        pendingAgentRewards[msg.sender] = 0;
        ainuToken.safeTransfer(msg.sender, pending);

        emit AgentRewardClaimed(msg.sender, pending);
    }

    /**
     * @notice Claim pending node operator rewards
     */
    function claimNodeRewards() external nonReentrant {
        uint256 pending = pendingNodeRewards[msg.sender];
        require(pending > 0, "No pending rewards");

        pendingNodeRewards[msg.sender] = 0;
        ainuToken.safeTransfer(msg.sender, pending);

        emit NodeRewardClaimed(msg.sender, pending);
    }

    /**
     * @notice Create a grant proposal (only owner or governance)
     * @param recipient Grant recipient
     * @param amount Grant amount
     * @param description Grant description
     * @return grantId New grant ID
     */
    function createGrant(
        address recipient,
        uint256 amount,
        string calldata description
    ) external onlyOwner returns (uint256 grantId) {
        require(recipient != address(0), "Invalid recipient");
        require(amount > 0, "Invalid amount");

        grantId = nextGrantId++;
        grants[grantId] = Grant({
            recipient: recipient,
            amount: amount,
            description: description,
            executed: false,
            timestamp: block.timestamp
        });

        emit GrantCreated(grantId, recipient, amount);
    }

    /**
     * @notice Execute a grant (only owner or governance)
     * @param grantId Grant to execute
     */
    function executeGrant(uint256 grantId) external onlyOwner nonReentrant {
        Grant storage grant = grants[grantId];
        require(!grant.executed, "Grant already executed");
        require(grant.recipient != address(0), "Invalid grant");

        grant.executed = true;
        ainuToken.safeTransfer(grant.recipient, grant.amount);

        emit GrantExecuted(grantId);
    }

    /**
     * @notice Buyback and burn AINU tokens
     * @param amount Amount to burn
     */
    function buybackAndBurn(uint256 amount) external onlyOwner nonReentrant {
        require(amount > 0, "Invalid amount");
        
        ainuToken.safeTransfer(address(0xdead), amount);
        totalBurned += amount;

        emit TokensBurned(amount);
    }

    /**
     * @notice Authorize or revoke revenue collector
     * @param collector Address to update
     * @param authorized Whether to authorize
     */
    function setAuthorizedCollector(address collector, bool authorized) external onlyOwner {
        authorizedCollectors[collector] = authorized;
        emit CollectorAuthorized(collector, authorized);
    }

    /**
     * @notice Get treasury balance
     * @return Current AINU balance
     */
    function getTreasuryBalance() external view returns (uint256) {
        return ainuToken.balanceOf(address(this));
    }

    /**
     * @notice Get pending rewards for an agent
     * @param agent Address to check
     * @return Pending reward amount
     */
    function getPendingAgentRewards(address agent) external view returns (uint256) {
        return pendingAgentRewards[agent];
    }

    /**
     * @notice Get pending rewards for a node operator
     * @param node Address to check
     * @return Pending reward amount
     */
    function getPendingNodeRewards(address node) external view returns (uint256) {
        return pendingNodeRewards[node];
    }

    /**
     * @notice Emergency withdraw (only owner, use with extreme caution)
     * @param token Token to withdraw
     * @param amount Amount to withdraw
     */
    function emergencyWithdraw(address token, uint256 amount) external onlyOwner {
        IERC20(token).safeTransfer(owner(), amount);
    }
}
