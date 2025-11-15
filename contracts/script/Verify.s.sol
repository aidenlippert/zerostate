// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/AINUToken.sol";
import "../src/AINUStaking.sol";
import "../src/AINUGovernance.sol";
import "../src/AINUTreasury.sol";

/**
 * @title Verify Script
 * @notice Verifies deployed contracts and their configuration
 * @dev Usage: forge script script/Verify.s.sol:VerifyScript --rpc-url $RPC_URL
 */
contract VerifyScript is Script {
    function run() public view {
        // Load addresses from environment or previous deployment
        address tokenAddr = vm.envAddress("TOKEN_ADDRESS");
        address stakingAddr = vm.envAddress("STAKING_ADDRESS");
        address governanceAddr = vm.envAddress("GOVERNANCE_ADDRESS");
        address treasuryAddr = vm.envAddress("TREASURY_ADDRESS");

        console.log("\n=== Verifying AINU Contracts ===\n");
        console.log("Network:", getNetworkName());
        console.log("Block Number:", block.number);
        console.log("Timestamp:", block.timestamp);

        // Load contracts
        AINUToken token = AINUToken(tokenAddr);
        AINUStaking staking = AINUStaking(stakingAddr);
        AINUGovernance governance = AINUGovernance(governanceAddr);
        AINUTreasury treasury = AINUTreasury(treasuryAddr);

        console.log("\n--- Token Verification ---");
        console.log("Address:", address(token));
        console.log("Name:", token.name());
        console.log("Symbol:", token.symbol());
        console.log("Total Supply:", token.totalSupply() / 1e18, "AINU");
        console.log("Burn Enabled:", token.burnOnTransferEnabled());
        console.log("Burn Rate:", token.BURN_RATE(), "bps");
        console.log("Owner:", token.owner());
        console.log("Paused:", token.paused());
        console.log("Staking Exempt:", token.exemptFromBurn(address(staking)));
        console.log("Treasury Exempt:", token.exemptFromBurn(address(treasury)));

        console.log("\n--- Staking Verification ---");
        console.log("Address:", address(staking));
        console.log("Token:", address(staking.ainuToken()));
        console.log("Owner:", staking.owner());
        console.log("Paused:", staking.paused());
        console.log("Total Staked:", staking.totalStaked() / 1e18, "AINU");
        console.log("Tier Basic:", staking.TIER_BASIC() / 1e18, "AINU");
        console.log("Tier Standard:", staking.TIER_STANDARD() / 1e18, "AINU");
        console.log("Tier Premium:", staking.TIER_PREMIUM() / 1e18, "AINU");

        console.log("\n--- Governance Verification ---");
        console.log("Address:", address(governance));
        console.log("Staking:", address(governance.staking()));
        console.log("Owner:", governance.owner());
        console.log("Proposal Threshold:", governance.PROPOSAL_THRESHOLD() / 1e18, "AINU");
        console.log("Voting Period:", governance.VOTING_PERIOD() / 1 days, "days");
        console.log("Timelock Period:", governance.TIMELOCK_PERIOD() / 1 hours, "hours");
        console.log("Next Proposal ID:", governance.nextProposalId());

        console.log("\n--- Treasury Verification ---");
        console.log("Address:", address(treasury));
        console.log("Token:", address(treasury.ainuToken()));
        console.log("Owner:", treasury.owner());
        console.log("Total Revenue:", treasury.totalRevenue() / 1e18, "AINU");
        console.log("Total Agent Rewards:", treasury.totalAgentRewards() / 1e18, "AINU");
        console.log("Total Node Rewards:", treasury.totalNodeRewards() / 1e18, "AINU");
        console.log("Total Burned:", treasury.totalBurned() / 1e18, "AINU");
        console.log("Treasury Balance:", token.balanceOf(address(treasury)) / 1e18, "AINU");
        console.log("Agent Share (bps):", treasury.AGENT_SHARE());
        console.log("Node Share (bps):", treasury.NODE_SHARE());

        console.log("\n--- Security Checks ---");
        require(token.owner() != address(0), "Token owner is zero address");
        require(staking.owner() != address(0), "Staking owner is zero address");
        require(governance.owner() != address(0), "Governance owner is zero address");
        require(treasury.owner() != address(0), "Treasury owner is zero address");
        require(token.totalSupply() == 10_000_000_000 * 10 ** 18, "Incorrect total supply");
        require(token.exemptFromBurn(address(staking)), "Staking not exempt from burns");
        require(token.exemptFromBurn(address(treasury)), "Treasury not exempt from burns");
        console.log("All security checks passed!");

        console.log("\n=== Verification Complete ===\n");
    }

    function getNetworkName() internal view returns (string memory) {
        if (block.chainid == 1) return "Ethereum Mainnet";
        if (block.chainid == 11155111) return "Sepolia Testnet";
        if (block.chainid == 5) return "Goerli Testnet";
        if (block.chainid == 31337) return "Local Hardhat/Anvil";
        return "Unknown Network";
    }
}
