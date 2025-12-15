// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract ContractForSLoadAndSStoreTest {
    uint256 public counter;
    uint256 public constant COUNTER_THRESHOLD_FOR_REVERT = 5;
    address public nextInstanceAddress;
    event CounterUpdated(uint256 counter);

    function setNextInstanceAddress(address _nextInstanceAddress) external {
        // Provide
        // 0x0000000000000000000000000000000000000000 to E
        // address of E for to D
        // address of D for to C
        // address of C for to B
        // address of B for to A
        nextInstanceAddress = _nextInstanceAddress;
    }

    function incrementAndCall(uint256 _counter) external {
        // Increment the counter
        counter = _counter + 1;
        // Emit event with the value of the counter
        emit CounterUpdated(counter);
        // Revert in case counter reaches the threshold
        require(counter < COUNTER_THRESHOLD_FOR_REVERT, "counter reached COUNTER_THRESHOLD_FOR_REVERT");
        // Invoke same function of the contract at nextInstanceAddress
        if (nextInstanceAddress != address(0)) {
            ContractForSLoadAndSStoreTest(nextInstanceAddress).incrementAndCall(counter);
        }
    }
}

/*
* EOA A.incrementAndCall(0) 
* A. 1 < 5 -> B.incrementAndCall(1)
* B. 2 < 5 -> C.incrementAndCall(2)
* C. 3 < 5 -> D.incrementAndCall(3)
* D. 4 < 5 -> E.incrementAndCall(4)
* E. 5 = 5 -> REVERT
*/