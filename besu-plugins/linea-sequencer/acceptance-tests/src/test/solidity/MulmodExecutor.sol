// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract MulmodExecutor {

    // This function executes a loop with a high number of mulmod operations.
    // The input parameter `iterations` controls how many times the loop runs.
    function executeMulmod(uint256 iterations) public pure returns (uint256) {
        uint256 result = 1;  // Start with 1 to avoid multiplying by zero
        uint256 result2 = 1;
        uint256 result3 = 1;
        uint256 result4 = 1;
        uint256 result5 = 1;
        for (uint256 i = 1; i <= iterations; i++) {
            // Perform the mulmod operation
            result = mulmod(result, i, 2**255);  // MULMOD opcode
            result2 = mulmod(result, i, 2**254);
            result3 = mulmod(result, i, 2**253);
            result4 = mulmod(result, i, 2**252);
            result5 = mulmod(result, i, 2**251);
        }

        return result;
    }
}