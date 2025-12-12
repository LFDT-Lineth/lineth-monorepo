// SPDX-License-Identifier: MIT
pragma solidity >=0.8.2 <0.9.0;

import "./MemoryOperations1.sol";

contract D {
    event End(string s);
    uint256 constant NUMBER_RANDOM_MEMORY_OPERATIONS = 20;

    function performMemoryOperations(address addressMO1) public {
        MemoryOperations1(addressMO1).execRandomMemoryOperations(NUMBER_RANDOM_MEMORY_OPERATIONS, 8888);
        emit End("end D");
    }
}
