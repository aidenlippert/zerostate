// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/AINUToken.sol";
import "../src/AINUStaking.sol";

contract AINUStakingRewardsTest is Test {
    AINUToken public token;
    AINUStaking public staking;

    address public owner = address(1);
    address public staker1 = address(2);
    address public staker2 = address(3);

    uint256 constant INITIAL_SUPPLY = 10_000_000_000 * 10 ** 18;
    uint256 constant STAKE_AMOUNT = 100_000 * 10 ** 18;

    function setUp() public {
        vm.startPrank(owner);
        token = new AINUToken(owner);
        staking = new AINUStaking(address(token), owner);

        // Exempt staking contract and stakers from burns
        token.setExemptFromBurn(address(staking), true);
        token.setExemptFromBurn(staker1, true);
        token.setExemptFromBurn(staker2, true);

        // Transfer tokens to stakers
        token.transfer(staker1, STAKE_AMOUNT * 2);
        token.transfer(staker2, STAKE_AMOUNT * 2);
        
        // Transfer rewards pool to staking contract
        token.transfer(address(staking), INITIAL_SUPPLY / 10); // 10% for rewards
        vm.stopPrank();
    }

    function testCalculatePendingRewardsNoLock() public {
        // Stake with no lock (< 3 months)
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 30 days); // 1 month
        vm.stopPrank();

        // Fast forward 365 days
        vm.warp(block.timestamp + 365 days);

        // Calculate expected rewards: 20% APR * 1.0x multiplier
        uint256 pendingRewards = staking.calculatePendingRewards(staker1);
        uint256 expectedRewards = (STAKE_AMOUNT * 20) / 100; // 20% of stake
        
        // Allow 1% tolerance for rounding
        assertApproxEqRel(pendingRewards, expectedRewards, 0.01e18);
    }

    function testCalculatePendingRewards3MonthLock() public {
        // Stake with 3 month lock (1.0x multiplier)
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 90 days);
        vm.stopPrank();

        // Fast forward 365 days
        vm.warp(block.timestamp + 365 days);

        // Calculate expected rewards: 20% APR * 1.0x = 20%
        uint256 pendingRewards = staking.calculatePendingRewards(staker1);
        uint256 expectedRewards = (STAKE_AMOUNT * 20) / 100;
        
        assertApproxEqRel(pendingRewards, expectedRewards, 0.01e18);
    }

    function testCalculatePendingRewards6MonthLock() public {
        // Stake with 6 month lock (1.5x multiplier)
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 180 days);
        vm.stopPrank();

        // Fast forward 365 days
        vm.warp(block.timestamp + 365 days);

        // Calculate expected rewards: 20% APR * 1.5x = 30%
        uint256 pendingRewards = staking.calculatePendingRewards(staker1);
        uint256 expectedRewards = (STAKE_AMOUNT * 30) / 100;
        
        assertApproxEqRel(pendingRewards, expectedRewards, 0.01e18);
    }

    function testCalculatePendingRewards12MonthLock() public {
        // Stake with 12 month lock (2.0x multiplier)
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 365 days);
        vm.stopPrank();

        // Fast forward 365 days
        vm.warp(block.timestamp + 365 days);

        // Calculate expected rewards: 20% APR * 2.0x = 40%
        uint256 pendingRewards = staking.calculatePendingRewards(staker1);
        uint256 expectedRewards = (STAKE_AMOUNT * 40) / 100;
        
        assertApproxEqRel(pendingRewards, expectedRewards, 0.01e18);
    }

    function testClaimRewards() public {
        // Stake
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 180 days); // 6 months = 1.5x
        vm.stopPrank();

        // Fast forward 1 year
        vm.warp(block.timestamp + 365 days);

        uint256 balanceBefore = token.balanceOf(staker1);
        uint256 pendingRewards = staking.calculatePendingRewards(staker1);

        // Claim rewards
        vm.prank(staker1);
        staking.claimRewards();

        uint256 balanceAfter = token.balanceOf(staker1);
        assertEq(balanceAfter - balanceBefore, pendingRewards);
    }

    function testClaimRewardsTwice() public {
        // Stake at timestamp 1
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 180 days);
        vm.stopPrank();

        uint256 firstClaimTime = block.timestamp + 180 days;
        vm.warp(firstClaimTime);

        // First claim after 6 months
        vm.prank(staker1);
        staking.claimRewards();

        // Fast forward another 6 months
        uint256 secondClaimTime = firstClaimTime + 180 days;
        vm.warp(secondClaimTime);

        uint256 balanceBefore = token.balanceOf(staker1);
        uint256 pendingRewards = staking.calculatePendingRewards(staker1);
        
        // Ensure rewards accumulated
        assertGt(pendingRewards, 0, "Should have pending rewards after 6 months");

        // Second claim should only get rewards since last claim
        vm.prank(staker1);
        staking.claimRewards();

        uint256 balanceAfter = token.balanceOf(staker1);
        assertEq(balanceAfter - balanceBefore, pendingRewards);
    }

    function testCompoundRewards() public {
        // Stake
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 365 days); // 12 months = 2.0x
        vm.stopPrank();

        // Fast forward 1 year
        vm.warp(block.timestamp + 365 days);

        uint256 stakeBefore = staking.totalStaked();
        uint256 pendingRewards = staking.calculatePendingRewards(staker1);

        // Compound rewards
        vm.prank(staker1);
        staking.compoundRewards();

        uint256 stakeAfter = staking.totalStaked();
        
        // Total staked should increase by rewards amount
        assertEq(stakeAfter - stakeBefore, pendingRewards);

        // User should have no pending rewards after compounding
        assertEq(staking.calculatePendingRewards(staker1), 0);
    }

    function testCompoundIncreasesTier() public {
        // Stake just below Premium tier (100K)
        uint256 belowPremium = 99_000 * 10 ** 18;
        vm.startPrank(staker1);
        token.approve(address(staking), belowPremium);
        staking.stake(belowPremium, 365 days); // 2.0x multiplier
        vm.stopPrank();

        // Check tier is Standard
        (, , , , AINUStaking.StakeTier tierBefore, , , , ) = staking.stakes(staker1);
        assertEq(uint(tierBefore), uint(AINUStaking.StakeTier.STANDARD));

        // Fast forward 1 year - rewards should push above 100K
        vm.warp(block.timestamp + 365 days);

        // Compound rewards
        vm.prank(staker1);
        staking.compoundRewards();

        // Check tier upgraded to Premium
        (, , , , AINUStaking.StakeTier tierAfter, , , , ) = staking.stakes(staker1);
        assertEq(uint(tierAfter), uint(AINUStaking.StakeTier.PREMIUM));
    }

    function testGetEffectiveAPR() public {
        // No lock: 20% * 1.0x = 20%
        assertEq(staking.getEffectiveAPR(0), 20_00);
        assertEq(staking.getEffectiveAPR(30 days), 20_00);
        
        // 3 months: 20% * 1.0x = 20%
        assertEq(staking.getEffectiveAPR(90 days), 20_00);
        
        // 6 months: 20% * 1.5x = 30%
        assertEq(staking.getEffectiveAPR(180 days), 30_00);
        
        // 12 months: 20% * 2.0x = 40%
        assertEq(staking.getEffectiveAPR(365 days), 40_00);
    }

    function testCannotClaimZeroRewards() public {
        // Stake
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 180 days);
        vm.stopPrank();

        // Try to claim immediately (no time passed)
        vm.prank(staker1);
        vm.expectRevert("No rewards to claim");
        staking.claimRewards();
    }

    function testCannotCompoundZeroRewards() public {
        // Stake
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 180 days);
        vm.stopPrank();

        // Try to compound immediately (no time passed)
        vm.prank(staker1);
        vm.expectRevert("No rewards to compound");
        staking.compoundRewards();
    }

    function testRewardsAccumulateOverTime() public {
        // Stake at timestamp 1
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 180 days); // 1.5x multiplier
        vm.stopPrank();

        // Warp to 60 days from start (5,184,001)
        vm.warp(5184001);
        
        uint256 rewards = staking.calculatePendingRewards(staker1);
        
        // Expected: 100k * 0.30 * (60/365) = 4,931.5 tokens = 4.9315e21 wei
        uint256 expected = 4931506849315068493150;
        assertApproxEqRel(rewards, expected, 0.01e18, "Rewards should match expected");
    }

    function testMultipleStakersIndependentRewards() public {
        // Staker1 stakes with 6-month lock (1.5x)
        vm.startPrank(staker1);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 180 days);
        vm.stopPrank();

        // Staker2 stakes with 12-month lock (2.0x)
        vm.startPrank(staker2);
        token.approve(address(staking), STAKE_AMOUNT);
        staking.stake(STAKE_AMOUNT, 365 days);
        vm.stopPrank();

        // Fast forward 1 year
        vm.warp(block.timestamp + 365 days);

        uint256 rewards1 = staking.calculatePendingRewards(staker1);
        uint256 rewards2 = staking.calculatePendingRewards(staker2);

        // Staker2 should have more rewards (2.0x vs 1.5x multiplier)
        assertGt(rewards2, rewards1);
        
        // Expected: staker1 = 30% (20% * 1.5x), staker2 = 40% (20% * 2.0x)
        uint256 expected1 = (STAKE_AMOUNT * 30) / 100;
        uint256 expected2 = (STAKE_AMOUNT * 40) / 100;
        
        assertApproxEqRel(rewards1, expected1, 0.01e18);
        assertApproxEqRel(rewards2, expected2, 0.01e18);
    }
}
