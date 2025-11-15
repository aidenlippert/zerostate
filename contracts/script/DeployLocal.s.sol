// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/AINUToken.sol";
import "../src/AINUStaking.sol";
import "../src/AINUGovernance.sol";
import "../src/AINUTreasury.sol";

/**
 * @title Deploy Local Script
 * @notice Deploys all AINU contracts locally with test data
 * @dev Usage: forge script script/DeployLocal.s.sol:DeployLocalScript --fork-url http://localhost:8545 --broadcast
 */
contract DeployLocalScript is Script {
    AINUToken public token;
    AINUStaking public staking;
    AINUGovernance public governance;
    AINUTreasury public treasury;

    address public deployer = address(0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266); // Anvil default
    address public alice = address(0x70997970C51812dc3A010C7d01b50e0d17dc79C8);
    address public bob = address(0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC);

    function run() public {
        vm.startBroadcast();

        console.log("\n=== Deploying AINU Contracts (Local) ===\n");

        // Deploy contracts
        token = new AINUToken(deployer);
        staking = new AINUStaking(address(token), deployer);
        governance = new AINUGovernance(address(staking), deployer);
        treasury = new AINUTreasury(address(token), deployer);

        console.log("Contracts deployed:");
        console.log("  Token:      ", address(token));
        console.log("  Staking:    ", address(staking));
        console.log("  Governance: ", address(governance));
        console.log("  Treasury:   ", address(treasury));

        // Configure
        token.setExemptFromBurn(address(staking), true);
        token.setExemptFromBurn(address(treasury), true);
        treasury.setAuthorizedCollector(deployer, true);

        // Disable burn for easier testing
        token.toggleBurnOnTransfer(false);
        console.log("\n  Burn on transfer: DISABLED (for local testing)");

        // Distribute tokens for testing
        console.log("\n=== Distributing Test Tokens ===\n");
        
        uint256 testAmount = 1_000_000 * 10 ** 18; // 1M tokens each
        token.transfer(alice, testAmount);
        token.transfer(bob, testAmount);
        
        console.log("  Alice received 1M AINU:", alice);
        console.log("  Bob received 1M AINU:", bob);

        vm.stopBroadcast();

        console.log("\n=== Local Setup Complete ===\n");
        console.log("You can now interact with the contracts");
        console.log("\nToken address:", address(token));
        console.log("Staking address:", address(staking));
    }
}
