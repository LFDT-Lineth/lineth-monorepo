// SPDX-License-Identifier: MIT
pragma solidity >=0.8.2 <0.9.0;

import "./Callee.sol";

contract Caller {

    function invokeCalleeFunctionWithEther(address calleeAddress, uint256 amount) external {
        Callee calleeContract = Callee(calleeAddress);
        // Transfer amount to the Callee contract from Caller balance
        calleeContract.calleeFunction{value: amount}();
    }


    function invokeOwnFunctionRecursively(uint256 iterations) public {
        if (iterations > 0) {
            this.invokeOwnFunctionRecursively(iterations - 1);
        }
    }

    // Function to receive Ether
    receive() external payable {}
}