// SPDX-License-Identifier: MIT
pragma solidity >=0.8.2 <0.9.0;

import "./B.sol";
import "./D.sol";
import "./MemoryOperations1.sol";

contract C {
    event End(string s);
    uint256 constant NUMBER_RANDOM_MEMORY_OPERATIONS = 20;

    function performMemoryOperations(address addressB, address addressD, address addressMO1) public {
        MemoryOperations1(addressMO1).execRandomMemoryOperations(NUMBER_RANDOM_MEMORY_OPERATIONS, 5555);
        B(addressB).performMemoryOperations(addressMO1);
        MemoryOperations1(addressMO1).execRandomMemoryOperations(NUMBER_RANDOM_MEMORY_OPERATIONS, 6666);
        D(addressD).performMemoryOperations(addressMO1);
        MemoryOperations1(addressMO1).execRandomMemoryOperations(NUMBER_RANDOM_MEMORY_OPERATIONS, 7777);
        emit End("end C");
    }
}
