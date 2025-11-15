// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/AINUToken.sol";

contract AINUTokenTest is Test {
    AINUToken public token;
    address public owner;
    address public user1;
    address public user2;

    function setUp() public {
        owner = address(this);
        user1 = address(0x1);
        user2 = address(0x2);
        
        token = new AINUToken(owner);
    }

    function testInitialSupply() public view {
        assertEq(token.totalSupply(), 10_000_000_000 * 10 ** 18);
        assertEq(token.balanceOf(owner), 10_000_000_000 * 10 ** 18);
    }

    function testBasicTransfer() public {
        uint256 amount = 1000 * 10 ** 18;
        
        // Disable burn for testing
        token.toggleBurnOnTransfer(false);
        
        token.transfer(user1, amount);
        assertEq(token.balanceOf(user1), amount);
    }

    function testBurnOnTransfer() public {
        uint256 amount = 1000 * 10 ** 18;
        uint256 burnAmount = (amount * 500) / 10_000; // 5%
        uint256 receiveAmount = amount - burnAmount;
        
        // Owner is exempt by default, so disable exemption for this test
        token.setExemptFromBurn(owner, false);
        
        // Burn is enabled by default
        token.transfer(user1, amount);
        
        assertEq(token.balanceOf(user1), receiveAmount);
        assertEq(token.balanceOf(address(0xdead)), burnAmount);
        assertEq(token.totalBurnedFromTransfers(), burnAmount);
    }

    function testBurnExemption() public {
        uint256 amount = 1000 * 10 ** 18;
        
        // Set owner as exempt (already exempt by default, but explicitly setting)
        token.setExemptFromBurn(owner, true);
        
        // Transfer to user1 (owner is exempt, no burn)
        token.transfer(user1, amount);
        assertEq(token.balanceOf(user1), amount);
        
        // Transfer from user1 to user2 (user1 not exempt, should burn)
        vm.prank(user1);
        token.transfer(user2, amount);
        
        uint256 burnAmount = (amount * 500) / 10_000;
        assertEq(token.balanceOf(user2), amount - burnAmount);
        assertEq(token.balanceOf(address(0xdead)), burnAmount);
    }

    function testManualBurn() public {
        uint256 amount = 1000 * 10 ** 18;
        uint256 initialSupply = token.totalSupply();
        
        token.burn(amount);
        
        assertEq(token.totalSupply(), initialSupply - amount);
        assertEq(token.totalBurnedManually(), amount);
    }

    function testCirculatingSupply() public {
        uint256 burnAmount = 1000 * 10 ** 18;
        uint256 initialSupply = 10_000_000_000 * 10 ** 18;
        
        token.burn(burnAmount);
        
        assertEq(token.circulatingSupply(), initialSupply - burnAmount);
    }

    function testPause() public {
        token.pause();
        
        vm.expectRevert();
        token.transfer(user1, 1000 * 10 ** 18);
    }

    function testToggleBurn() public {
        uint256 amount = 1000 * 10 ** 18;
        
        // Disable burn
        token.toggleBurnOnTransfer(false);
        token.transfer(user1, amount);
        assertEq(token.balanceOf(user1), amount);
        
        // Enable burn
        token.toggleBurnOnTransfer(true);
        vm.prank(user1);
        token.transfer(user2, amount);
        
        uint256 burnAmount = (amount * 500) / 10_000;
        assertEq(token.balanceOf(user2), amount - burnAmount);
    }

    function testOnlyOwnerFunctions() public {
        vm.prank(user1);
        vm.expectRevert();
        token.toggleBurnOnTransfer(false);
        
        vm.prank(user1);
        vm.expectRevert();
        token.setExemptFromBurn(user2, true);
        
        vm.prank(user1);
        vm.expectRevert();
        token.pause();
    }
}
