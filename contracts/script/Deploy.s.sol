// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/AINUToken.sol";
import "../src/AINUStaking.sol";
import "../src/AINUGovernance.sol";
import "../src/AINUTreasury.sol";

/**
 * @title Deploy Script
 * @notice Deploys all AINU contracts in correct order with proper configuration
 * @dev Usage:
 *   Testnet: forge script script/Deploy.s.sol:DeployScript --rpc-url $SEPOLIA_RPC_URL --broadcast --verify
 *   Mainnet: forge script script/Deploy.s.sol:DeployScript --rpc-url $MAINNET_RPC_URL --broadcast --verify
 */
contract DeployScript is Script {
    // Deployment addresses will be saved here
    AINUToken public token;
    AINUStaking public staking;
    AINUGovernance public governance;
    AINUTreasury public treasury;

    // Configuration
    address public deployer;
    address public multisig; // For mainnet governance control

    function setUp() public {
        // Get deployer from private key
        deployer = vm.addr(vm.envUint("PRIVATE_KEY"));
        
        // Multisig address (replace with actual multisig for mainnet)
        multisig = vm.envOr("MULTISIG_ADDRESS", deployer);
        
        console.log("Deployer:", deployer);
        console.log("Multisig:", multisig);
    }

    function run() public {
        vm.startBroadcast(vm.envUint("PRIVATE_KEY"));

        console.log("\n=== Deploying AINU Contracts ===\n");

        // 1. Deploy Token
        console.log("1. Deploying AINUToken...");
        token = new AINUToken(deployer);
        console.log("   AINUToken deployed at:", address(token));
        console.log("   Total Supply:", token.totalSupply() / 1e18, "AINU");

        // 2. Deploy Staking
        console.log("\n2. Deploying AINUStaking...");
        staking = new AINUStaking(address(token), deployer);
        console.log("   AINUStaking deployed at:", address(staking));

        // 3. Deploy Governance
        console.log("\n3. Deploying AINUGovernance...");
        governance = new AINUGovernance(address(staking), deployer);
        console.log("   AINUGovernance deployed at:", address(governance));

        // 4. Deploy Treasury
        console.log("\n4. Deploying AINUTreasury...");
        treasury = new AINUTreasury(address(token), deployer);
        console.log("   AINUTreasury deployed at:", address(treasury));

        // 5. Configure contracts
        console.log("\n=== Configuring Contracts ===\n");

        // Exempt staking and treasury from burns
        console.log("5. Setting burn exemptions...");
        token.setExemptFromBurn(address(staking), true);
        token.setExemptFromBurn(address(treasury), true);
        console.log("   Staking exempt from burns: true");
        console.log("   Treasury exempt from burns: true");

        // Authorize treasury as collector (in production, this would be the task execution contract)
        console.log("\n6. Authorizing treasury as collector...");
        treasury.setAuthorizedCollector(deployer, true); // Temporarily authorize deployer for testing
        console.log("   Deployer authorized as collector");

        // 7. Transfer ownership to multisig (for mainnet)
        if (multisig != deployer && block.chainid == 1) {
            console.log("\n7. Transferring ownership to multisig...");
            token.transferOwnership(multisig);
            staking.transferOwnership(multisig);
            governance.transferOwnership(multisig);
            treasury.transferOwnership(multisig);
            console.log("   All contracts owned by:", multisig);
        } else {
            console.log("\n7. Skipping ownership transfer (testnet or deployer == multisig)");
        }

        vm.stopBroadcast();

        // Print deployment summary
        console.log("\n=== Deployment Summary ===\n");
        console.log("Network:", getNetworkName());
        console.log("Deployer:", deployer);
        console.log("Owner:", multisig);
        console.log("\nContract Addresses:");
        console.log("  AINUToken:      ", address(token));
        console.log("  AINUStaking:    ", address(staking));
        console.log("  AINUGovernance: ", address(governance));
        console.log("  AINUTreasury:   ", address(treasury));
        
        console.log("\nConfiguration:");
        console.log("  Burn on transfer:", token.burnOnTransferEnabled());
        console.log("  Burn rate (bps):", token.BURN_RATE());
        console.log("  Total supply:", token.totalSupply() / 1e18, "AINU");
        
        console.log("\nNext Steps:");
        console.log("  1. Verify contracts on Etherscan");
        console.log("  2. Set up liquidity pools (exempt from burns)");
        console.log("  3. Authorize task execution contract as treasury collector");
        console.log("  4. Transfer ownership to multisig (if not done)");
        
        // Save deployment addresses to file
        saveDeploymentAddresses();
    }

    function getNetworkName() internal view returns (string memory) {
        if (block.chainid == 1) return "Ethereum Mainnet";
        if (block.chainid == 11155111) return "Sepolia Testnet";
        if (block.chainid == 5) return "Goerli Testnet";
        if (block.chainid == 31337) return "Local Hardhat/Anvil";
        return "Unknown Network";
    }

    function saveDeploymentAddresses() internal {
        string memory json = string(abi.encodePacked(
            '{\n',
            '  "network": "', getNetworkName(), '",\n',
            '  "chainId": ', vm.toString(block.chainid), ',\n',
            '  "deployer": "', vm.toString(deployer), '",\n',
            '  "owner": "', vm.toString(multisig), '",\n',
            '  "contracts": {\n',
            '    "AINUToken": "', vm.toString(address(token)), '",\n',
            '    "AINUStaking": "', vm.toString(address(staking)), '",\n',
            '    "AINUGovernance": "', vm.toString(address(governance)), '",\n',
            '    "AINUTreasury": "', vm.toString(address(treasury)), '"\n',
            '  },\n',
            '  "timestamp": ', vm.toString(block.timestamp), '\n',
            '}'
        ));

        string memory filename = string(abi.encodePacked(
            "deployments/",
            vm.toString(block.chainid),
            "-",
            vm.toString(block.timestamp),
            ".json"
        ));

        vm.writeFile(filename, json);
        console.log("\nDeployment addresses saved to:", filename);
    }
}
