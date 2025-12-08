// SPDX-License-Identifier: MIT
pragma solidity >=0.8.2 <0.9.0;

import "./B.sol";
import "./C.sol";
import "./MemoryOperations1.sol";

contract A {
    event End(string s);
    uint256 constant NUMBER_RANDOM_MEMORY_OPERATIONS = 20;

    function performMemoryOperations(address addressB,address addressC,address addressD, address addressMO1, address addressMO2) public {
        MemoryOperations1(addressMO1).execRandomMemoryOperations(NUMBER_RANDOM_MEMORY_OPERATIONS, 1111);
        B(addressB).performMemoryOperations(addressMO1);
        MemoryOperations1(addressMO1).execRandomMemoryOperations(NUMBER_RANDOM_MEMORY_OPERATIONS, 2222);
        C(addressC).performMemoryOperations(addressB, addressD, addressMO1);
        MemoryOperations1(addressMO1).execRandomMemoryOperations(NUMBER_RANDOM_MEMORY_OPERATIONS, 3333);

        // Return is executed just in the very end since it kills the current execution environment
        MemoryOperations1(addressMO1).returndatacopyExecAfterReturn(addressMO2);

        emit End("end A");
    }
}
