// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Permit.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/Pausable.sol";

/**
 * @title AINU Token
 * @notice ERC-20 token for the Ainur decentralized agent marketplace
 * @dev Implements:
 *   - Fixed supply (10 billion tokens)
 *   - Burnable (deflationary mechanism)
 *   - Permit (EIP-2612 gasless approvals)
 *   - Pausable (emergency stop)
 *   - 5% burn on transfers (can be toggled)
 */
contract AINUToken is ERC20, ERC20Burnable, ERC20Permit, Ownable, Pausable {
    /// @notice Total supply: 10 billion AINU with 18 decimals
    uint256 public constant TOTAL_SUPPLY = 10_000_000_000 * 10 ** 18;

    /// @notice Burn rate: 5% of each transfer (0.05 = 500 basis points)
    uint256 public constant BURN_RATE = 500; // 5% in basis points (500/10000)
    uint256 public constant BASIS_POINTS = 10_000;

    /// @notice Whether automatic burning on transfers is enabled
    bool public burnOnTransferEnabled;

    /// @notice Addresses exempt from burn (e.g., DEX pools, staking contracts)
    mapping(address => bool) public exemptFromBurn;

    /// @notice Total amount burned through transfers
    uint256 public totalBurnedFromTransfers;

    /// @notice Total amount burned through manual burns
    uint256 public totalBurnedManually;

    event BurnOnTransferToggled(bool enabled);
    event ExemptionUpdated(address indexed account, bool exempt);
    event TokensBurned(address indexed from, uint256 amount, bool isTransferBurn);

    constructor(address initialOwner) ERC20("AINU", "AINU") ERC20Permit("AINU") Ownable(initialOwner) {
        _mint(initialOwner, TOTAL_SUPPLY);
        burnOnTransferEnabled = true;
        
        // Owner is exempt from burns by default
        exemptFromBurn[initialOwner] = true;
    }

    /**
     * @notice Toggle automatic burning on transfers
     * @param enabled Whether burning should be enabled
     */
    function toggleBurnOnTransfer(bool enabled) external onlyOwner {
        burnOnTransferEnabled = enabled;
        emit BurnOnTransferToggled(enabled);
    }

    /**
     * @notice Set burn exemption status for an address
     * @param account Address to update
     * @param exempt Whether the address should be exempt from burns
     */
    function setExemptFromBurn(address account, bool exempt) external onlyOwner {
        exemptFromBurn[account] = exempt;
        emit ExemptionUpdated(account, exempt);
    }

    /**
     * @notice Pause all token transfers (emergency use only)
     */
    function pause() external onlyOwner {
        _pause();
    }

    /**
     * @notice Unpause token transfers
     */
    function unpause() external onlyOwner {
        _unpause();
    }

    /**
     * @notice Get total amount burned (transfers + manual)
     * @return Total burned tokens
     */
    function totalBurned() public view returns (uint256) {
        return totalBurnedFromTransfers + totalBurnedManually;
    }

    /**
     * @notice Get circulating supply (total supply - burned)
     * @return Current circulating supply
     */
    function circulatingSupply() public view returns (uint256) {
        return TOTAL_SUPPLY - totalBurned();
    }

    /**
     * @notice Override burn to track manual burns
     * @param amount Amount to burn
     */
    function burn(uint256 amount) public override {
        super.burn(amount);
        totalBurnedManually += amount;
        emit TokensBurned(msg.sender, amount, false);
    }

    /**
     * @notice Override burnFrom to track manual burns
     * @param account Account to burn from
     * @param amount Amount to burn
     */
    function burnFrom(address account, uint256 amount) public override {
        super.burnFrom(account, amount);
        totalBurnedManually += amount;
        emit TokensBurned(account, amount, false);
    }

    /**
     * @notice Override _update to implement burn-on-transfer
     * @dev Called by all transfer functions (transfer, transferFrom, mint, burn)
     */
    function _update(address from, address to, uint256 value) internal override whenNotPaused {
        // Skip burn logic for mints, burns, and dead address
        if (from == address(0) || to == address(0) || to == address(0xdead)) {
            super._update(from, to, value);
            return;
        }

        // Skip burn if disabled or sender/recipient is exempt
        if (!burnOnTransferEnabled || exemptFromBurn[from] || exemptFromBurn[to]) {
            super._update(from, to, value);
            return;
        }

        // Calculate burn amount (5% of transfer)
        uint256 burnAmount = (value * BURN_RATE) / BASIS_POINTS;
        uint256 transferAmount = value - burnAmount;

        // Burn tokens to dead address
        if (burnAmount > 0) {
            super._update(from, address(0xdead), burnAmount);
            totalBurnedFromTransfers += burnAmount;
            emit TokensBurned(from, burnAmount, true);
        }

        // Transfer remaining tokens
        super._update(from, to, transferAmount);
    }
}
