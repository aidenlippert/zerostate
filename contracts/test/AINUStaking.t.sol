// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/AINUToken.sol";
import "../src/AINUStaking.sol";

contract AINUStakingTest is Test {
    AINUToken public token;
    AINUStaking public staking;
    address public owner;
    address public agent1;
    address public agent2;

    function setUp() public {
        owner = address(this);
        agent1 = address(0x1);
        agent2 = address(0x2);
        
        token = new AINUToken(owner);
        staking = new AINUStaking(address(token), owner);
        
        // Disable burn for easier testing
        token.toggleBurnOnTransfer(false);
        
        // Give agents tokens
        token.transfer(agent1, 200_000 * 10 ** 18);
        token.transfer(agent2, 200_000 * 10 ** 18);
    }

    function testBasicStake() public {
        uint256 stakeAmount = 10_000 * 10 ** 18;
        
        vm.startPrank(agent1);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 365 days);
        vm.stopPrank();
        
        (uint256 amount, , , , AINUStaking.StakeTier tier, , , , ) = staking.stakes(agent1);
        assertEq(amount, stakeAmount);
        assertEq(uint(tier), uint(AINUStaking.StakeTier.STANDARD));
    }

    function testStakingTiers() public {
        // Basic tier
        vm.startPrank(agent1);
        token.approve(address(staking), 1_000 * 10 ** 18);
        staking.stake(1_000 * 10 ** 18, 0);
        vm.stopPrank();
        
        (, , , , AINUStaking.StakeTier tier1, , , , ) = staking.stakes(agent1);
        assertEq(uint(tier1), uint(AINUStaking.StakeTier.BASIC));
        
        // Premium tier
        vm.startPrank(agent2);
        token.approve(address(staking), 100_000 * 10 ** 18);
        staking.stake(100_000 * 10 ** 18, 0);
        vm.stopPrank();
        
        (, , , , AINUStaking.StakeTier tier2, , , , ) = staking.stakes(agent2);
        assertEq(uint(tier2), uint(AINUStaking.StakeTier.PREMIUM));
    }

    function testVotingPower() public {
        uint256 stakeAmount = 10_000 * 10 ** 18;
        
        // Stake with no lock (1x multiplier)
        vm.startPrank(agent1);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 0);
        vm.stopPrank();
        
        uint256 votingPower1 = staking.getVotingPower(agent1);
        assertEq(votingPower1, stakeAmount); // 1x
        
        // Stake with 4-year lock (4x multiplier)
        vm.startPrank(agent2);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 4 * 365 days);
        vm.stopPrank();
        
        uint256 votingPower2 = staking.getVotingPower(agent2);
        assertGt(votingPower2, votingPower1 * 3); // Should be close to 4x
    }

    function testAddToStake() public {
        uint256 initialStake = 1_000 * 10 ** 18;
        uint256 additionalStake = 10_000 * 10 ** 18;
        
        // Initial stake (Basic tier)
        vm.startPrank(agent1);
        token.approve(address(staking), initialStake + additionalStake);
        staking.stake(initialStake, 0);
        
        (, , , , AINUStaking.StakeTier tier1, , , , ) = staking.stakes(agent1);
        assertEq(uint(tier1), uint(AINUStaking.StakeTier.BASIC));
        
        // Add to stake (upgrade to Standard tier)
        staking.addToStake(additionalStake);
        vm.stopPrank();
        
        (uint256 amount, , , , AINUStaking.StakeTier tier2, , , , ) = staking.stakes(agent1);
        assertEq(amount, initialStake + additionalStake);
        assertEq(uint(tier2), uint(AINUStaking.StakeTier.STANDARD));
    }

    function testUnstake() public {
        uint256 stakeAmount = 10_000 * 10 ** 18;
        
        vm.startPrank(agent1);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 365 days);
        
        // Try to unstake before lock expires
        vm.expectRevert("Stake still locked");
        staking.unstake();
        
        // Fast forward time
        vm.warp(block.timestamp + 366 days);
        
        // Now unstake should work
        uint256 balanceBefore = token.balanceOf(agent1);
        staking.unstake();
        uint256 balanceAfter = token.balanceOf(agent1);
        
        assertEq(balanceAfter - balanceBefore, stakeAmount);
        vm.stopPrank();
    }

    function testSlashing() public {
        uint256 stakeAmount = 10_000 * 10 ** 18;
        
        vm.startPrank(agent1);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 0);
        vm.stopPrank();
        
        // Authorize owner as slasher
        staking.setAuthorizedSlasher(owner, true);
        
        // Slash 5% for downtime
        staking.slash(agent1, 500, "Downtime >10%");
        
        (uint256 amount, , , , , , uint256 slashedAmount, , ) = staking.stakes(agent1);
        assertEq(slashedAmount, (stakeAmount * 500) / 10_000);
        
        // Fast forward and unstake
        vm.warp(block.timestamp + 1);
        vm.prank(agent1);
        staking.unstake();
        
        // Should receive full amount minus slashed
        assertEq(token.balanceOf(agent1), 200_000 * 10 ** 18 - (stakeAmount * 500) / 10_000);
    }

    function testTotalStaked() public {
        uint256 stake1 = 10_000 * 10 ** 18;
        uint256 stake2 = 20_000 * 10 ** 18;
        
        vm.prank(agent1);
        token.approve(address(staking), stake1);
        vm.prank(agent1);
        staking.stake(stake1, 0);
        
        vm.prank(agent2);
        token.approve(address(staking), stake2);
        vm.prank(agent2);
        staking.stake(stake2, 0);
        
        assertEq(staking.totalStaked(), stake1 + stake2);
    }

    function testCannotStakeBelowMinimum() public {
        uint256 tooSmall = 500 * 10 ** 18;
        
        vm.startPrank(agent1);
        token.approve(address(staking), tooSmall);
        
        vm.expectRevert("Amount below minimum stake");
        staking.stake(tooSmall, 0);
        vm.stopPrank();
    }

    function testCannotStakeTwice() public {
        uint256 stakeAmount = 10_000 * 10 ** 18;
        
        vm.startPrank(agent1);
        token.approve(address(staking), stakeAmount * 2);
        staking.stake(stakeAmount, 0);
        
        vm.expectRevert("Already staking");
        staking.stake(stakeAmount, 0);
        vm.stopPrank();
    }
}
