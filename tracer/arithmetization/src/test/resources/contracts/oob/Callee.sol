// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Callee {
    event EtherReceived(address sender, uint256 amount);

    function calleeFunction() external payable {
        emit EtherReceived(msg.sender, msg.value);
    }
}