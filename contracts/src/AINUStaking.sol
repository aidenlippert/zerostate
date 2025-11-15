// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/utils/Pausable.sol";

/**
 * @title AINU Staking Contract
 * @notice Allows agents to stake AINU tokens to participate in the marketplace
 * @dev Implements:
 *   - Tiered staking (Basic, Standard, Premium)
 *   - Time-weighted voting power (up to 4x for 4-year stakes)
 *   - Slashing for malicious behavior
 *   - Reward distribution
 */
contract AINUStaking is Ownable, ReentrancyGuard, Pausable {
    using SafeERC20 for IERC20;

    /// @notice AINU token contract
    IERC20 public immutable ainuToken;

    /// @notice Staking tiers
    enum StakeTier {
        NONE,
        BASIC,      // 1,000 AINU
        STANDARD,   // 10,000 AINU
        PREMIUM     // 100,000 AINU
    }

    /// @notice Minimum stake amounts for each tier
    uint256 public constant TIER_BASIC = 1_000 * 10 ** 18;
    uint256 public constant TIER_STANDARD = 10_000 * 10 ** 18;
    uint256 public constant TIER_PREMIUM = 100_000 * 10 ** 18;

    /// @notice Maximum time weight multiplier (4x for 4 years)
    uint256 public constant MAX_TIME_WEIGHT = 4;
    uint256 public constant SECONDS_PER_YEAR = 365 days;

    /// @notice Staking APR (20% = 2000 basis points) - Updated from 12% based on economic simulation
    uint256 public constant STAKING_APR = 20_00; // 20% APR
    
    /// @notice Time-based reward multipliers (based on lock duration)
    uint256 public constant MULTIPLIER_3_MONTHS = 10_000;  // 1.0x (100%)
    uint256 public constant MULTIPLIER_6_MONTHS = 15_000;  // 1.5x (150%)
    uint256 public constant MULTIPLIER_12_MONTHS = 20_000; // 2.0x (200%)
    
    /// @notice Lock duration thresholds
    uint256 public constant LOCK_3_MONTHS = 90 days;
    uint256 public constant LOCK_6_MONTHS = 180 days;
    uint256 public constant LOCK_12_MONTHS = 365 days;

    /// @notice Slashing percentages (in basis points)
    uint256 public constant SLASH_DOWNTIME = 500; // 5% for >10% downtime
    uint256 public constant SLASH_FAILED_TASK = 100; // 1% for failed task
    uint256 public constant SLASH_MALICIOUS = 10_000; // 100% for malicious behavior
    uint256 public constant BASIS_POINTS = 10_000;

    /// @notice Stake information
    struct Stake {
        uint256 amount;          // Amount staked
        uint256 timestamp;       // When stake was created
        uint256 lockDuration;    // Lock duration in seconds
        uint256 unlockTime;      // When stake can be withdrawn
        StakeTier tier;          // Staking tier
        bool isSlashed;          // Whether stake has been slashed
        uint256 slashedAmount;   // Amount slashed
        uint256 lastRewardClaim; // Last time rewards were claimed
        uint256 accumulatedRewards; // Rewards not yet claimed
    }

    /// @notice Staker address => Stake info
    mapping(address => Stake) public stakes;

    /// @notice Total amount staked across all users
    uint256 public totalStaked;

    /// @notice Total amount slashed
    uint256 public totalSlashed;

    /// @notice Authorized slashers (contracts that can slash stakes)
    mapping(address => bool) public authorizedSlashers;

    event Staked(address indexed user, uint256 amount, StakeTier tier, uint256 lockDuration);
    event Unstaked(address indexed user, uint256 amount);
    event Slashed(address indexed user, uint256 amount, string reason);
    event SlasherAuthorized(address indexed slasher, bool authorized);
    event TierUpgraded(address indexed user, StakeTier oldTier, StakeTier newTier);
    event RewardsClaimed(address indexed user, uint256 amount);
    event RewardsCompounded(address indexed user, uint256 amount);

    constructor(address _ainuToken, address initialOwner) Ownable(initialOwner) {
        ainuToken = IERC20(_ainuToken);
    }

    /**
     * @notice Stake AINU tokens for a specific duration
     * @param amount Amount to stake
     * @param lockDuration Duration to lock tokens (in seconds)
     */
    function stake(uint256 amount, uint256 lockDuration) external nonReentrant whenNotPaused {
        require(amount >= TIER_BASIC, "Amount below minimum stake");
        require(lockDuration <= 4 * SECONDS_PER_YEAR, "Lock duration too long");
        require(stakes[msg.sender].amount == 0, "Already staking");

        // Determine tier
        StakeTier tier = _getTier(amount);

        // Transfer tokens
        ainuToken.safeTransferFrom(msg.sender, address(this), amount);

        // Create stake
        stakes[msg.sender] = Stake({
            amount: amount,
            timestamp: block.timestamp,
            lockDuration: lockDuration,
            unlockTime: block.timestamp + lockDuration,
            tier: tier,
            isSlashed: false,
            slashedAmount: 0,
            lastRewardClaim: block.timestamp,
            accumulatedRewards: 0
        });

        totalStaked += amount;

        emit Staked(msg.sender, amount, tier, lockDuration);
    }

    /**
     * @notice Add more tokens to existing stake
     * @param amount Amount to add
     */
    function addToStake(uint256 amount) external nonReentrant whenNotPaused {
        Stake storage userStake = stakes[msg.sender];
        require(userStake.amount > 0, "No existing stake");
        require(!userStake.isSlashed, "Stake has been slashed");

        // Transfer tokens
        ainuToken.safeTransferFrom(msg.sender, address(this), amount);

        // Update stake
        StakeTier oldTier = userStake.tier;
        userStake.amount += amount;
        userStake.tier = _getTier(userStake.amount);
        totalStaked += amount;

        if (userStake.tier != oldTier) {
            emit TierUpgraded(msg.sender, oldTier, userStake.tier);
        }

        emit Staked(msg.sender, amount, userStake.tier, userStake.lockDuration);
    }

    /**
     * @notice Unstake tokens after lock period
     */
    function unstake() external nonReentrant {
        Stake storage userStake = stakes[msg.sender];
        require(userStake.amount > 0, "No stake found");
        require(block.timestamp >= userStake.unlockTime, "Stake still locked");

        uint256 amountToReturn = userStake.amount - userStake.slashedAmount;
        totalStaked -= userStake.amount;

        // Delete stake before transfer (CEI pattern)
        delete stakes[msg.sender];

        // Transfer tokens back
        ainuToken.safeTransfer(msg.sender, amountToReturn);

        emit Unstaked(msg.sender, amountToReturn);
    }

    /**
     * @notice Slash a staker's tokens for misbehavior
     * @param staker Address to slash
     * @param percentage Percentage to slash (in basis points)
     * @param reason Reason for slashing
     */
    function slash(address staker, uint256 percentage, string calldata reason) external {
        require(authorizedSlashers[msg.sender], "Not authorized to slash");
        require(percentage <= BASIS_POINTS, "Invalid percentage");

        Stake storage userStake = stakes[staker];
        require(userStake.amount > 0, "No stake found");

        uint256 slashAmount = (userStake.amount * percentage) / BASIS_POINTS;
        userStake.slashedAmount += slashAmount;
        userStake.isSlashed = true;
        totalSlashed += slashAmount;

        // Burn slashed tokens by transferring to dead address
        ainuToken.safeTransfer(address(0xdead), slashAmount);

        emit Slashed(staker, slashAmount, reason);
    }

    /**
     * @notice Get voting power for a staker
     * @param staker Address to check
     * @return Voting power (stake amount × time weight)
     */
    function getVotingPower(address staker) external view returns (uint256) {
        Stake memory userStake = stakes[staker];
        if (userStake.amount == 0 || userStake.isSlashed) {
            return 0;
        }

        uint256 timeWeight = _calculateTimeWeight(userStake.lockDuration);
        return (userStake.amount * timeWeight) / 1e18;
    }

    /**
     * @notice Get stake information for a user
     * @param staker Address to check
     * @return Stake details
     */
    function getStake(address staker) external view returns (Stake memory) {
        return stakes[staker];
    }

    /**
     * @notice Authorize or revoke slashing permission
     * @param slasher Address to update
     * @param authorized Whether to authorize
     */
    function setAuthorizedSlasher(address slasher, bool authorized) external onlyOwner {
        authorizedSlashers[slasher] = authorized;
        emit SlasherAuthorized(slasher, authorized);
    }

    /**
     * @notice Pause staking (emergency use only)
     */
    function pause() external onlyOwner {
        _pause();
    }

    /**
     * @notice Unpause staking
     */
    function unpause() external onlyOwner {
        _unpause();
    }

    /**
     * @notice Calculate tier based on stake amount
     * @param amount Stake amount
     * @return Corresponding tier
     */
    function _getTier(uint256 amount) internal pure returns (StakeTier) {
        if (amount >= TIER_PREMIUM) return StakeTier.PREMIUM;
        if (amount >= TIER_STANDARD) return StakeTier.STANDARD;
        if (amount >= TIER_BASIC) return StakeTier.BASIC;
        return StakeTier.NONE;
    }

    /**
     * @notice Calculate time weight multiplier (1x to 4x)
     * @param lockDuration Lock duration in seconds
     * @return Time weight (scaled by 1e18)
     */
    function _calculateTimeWeight(uint256 lockDuration) internal pure returns (uint256) {
        if (lockDuration == 0) return 1e18; // 1x
        
        // Linear scaling: 1x at 0 years, 4x at 4 years
        uint256 maxLock = 4 * SECONDS_PER_YEAR;
        uint256 weight = 1e18 + ((lockDuration * 3e18) / maxLock);
        
        return weight > 4e18 ? 4e18 : weight;
    }

    /**
     * @notice Calculate pending rewards for a staker
     * @param staker Address to check
     * @return Pending reward amount
     */
    function calculatePendingRewards(address staker) public view returns (uint256) {
        Stake memory userStake = stakes[staker];
        if (userStake.amount == 0 || userStake.isSlashed) {
            return 0;
        }

        // Calculate time elapsed since last claim
        uint256 timeElapsed = block.timestamp - userStake.lastRewardClaim;
        
        // Calculate base APR rewards (per second)
        // Formula: (amount * APR * timeElapsed) / (BASIS_POINTS * SECONDS_PER_YEAR)
        uint256 baseRewards = (userStake.amount * STAKING_APR * timeElapsed) / (BASIS_POINTS * SECONDS_PER_YEAR);
        
        // Apply time-based multiplier based on lock duration
        uint256 multiplier = _getLockMultiplier(userStake.lockDuration);
        uint256 totalRewards = (baseRewards * multiplier) / BASIS_POINTS;
        
        return totalRewards + userStake.accumulatedRewards;
    }

    /**
     * @notice Get reward multiplier based on lock duration
     * @param lockDuration Lock duration in seconds
     * @return Multiplier in basis points (10000 = 1x, 15000 = 1.5x, 20000 = 2x)
     */
    function _getLockMultiplier(uint256 lockDuration) internal pure returns (uint256) {
        if (lockDuration >= LOCK_12_MONTHS) {
            return MULTIPLIER_12_MONTHS; // 2.0x for 12+ months
        } else if (lockDuration >= LOCK_6_MONTHS) {
            return MULTIPLIER_6_MONTHS;  // 1.5x for 6+ months
        } else if (lockDuration >= LOCK_3_MONTHS) {
            return MULTIPLIER_3_MONTHS;  // 1.0x for 3+ months
        } else {
            return BASIS_POINTS;          // 1.0x for less than 3 months
        }
    }

    /**
     * @notice Claim accumulated staking rewards
     */
    function claimRewards() external nonReentrant whenNotPaused {
        Stake storage userStake = stakes[msg.sender];
        require(userStake.amount > 0, "No stake found");
        require(!userStake.isSlashed, "Stake has been slashed");

        uint256 rewards = calculatePendingRewards(msg.sender);
        require(rewards > 0, "No rewards to claim");

        // Update state
        userStake.lastRewardClaim = block.timestamp;
        userStake.accumulatedRewards = 0;

        // Transfer rewards
        ainuToken.safeTransfer(msg.sender, rewards);

        emit RewardsClaimed(msg.sender, rewards);
    }

    /**
     * @notice Compound rewards into stake (auto-stake rewards)
     * @dev This increases the staked amount with accumulated rewards
     */
    function compoundRewards() external nonReentrant whenNotPaused {
        Stake storage userStake = stakes[msg.sender];
        require(userStake.amount > 0, "No stake found");
        require(!userStake.isSlashed, "Stake has been slashed");

        uint256 rewards = calculatePendingRewards(msg.sender);
        require(rewards > 0, "No rewards to compound");

        // Update state
        userStake.lastRewardClaim = block.timestamp;
        userStake.accumulatedRewards = 0;
        userStake.amount += rewards;
        totalStaked += rewards;

        // Update tier if needed
        StakeTier oldTier = userStake.tier;
        userStake.tier = _getTier(userStake.amount);
        
        if (userStake.tier != oldTier) {
            emit TierUpgraded(msg.sender, oldTier, userStake.tier);
        }

        emit RewardsCompounded(msg.sender, rewards);
    }

    /**
     * @notice Get expected APR for a given lock duration
     * @param lockDuration Lock duration in seconds
     * @return Effective APR in basis points (e.g., 2000 = 20%, 3000 = 30%)
     */
    function getEffectiveAPR(uint256 lockDuration) external pure returns (uint256) {
        uint256 multiplier = _getLockMultiplier(lockDuration);
        return (STAKING_APR * multiplier) / BASIS_POINTS;
    }
}

