// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract ContractForSLoadAndSStoreRecursiveTest {
    uint256 public counter = 0;
    uint256 public constant COUNTER_THRESHOLD = 5;
    event CounterUpdated(uint256 counter);
    
    function incrementAndCall(bool _rootReverts) public {
        // Increment the counter
        counter = counter + 1;
        // Emit event with the value of the counter
        emit CounterUpdated(counter);
        // Recursive call       
        if (counter < COUNTER_THRESHOLD){
            incrementAndCall(_rootReverts);
        }
        require(_rootReverts? counter % 2 == 0 : counter % 2 != 0, _rootReverts? "REVERT due to odd counter" : "REVERT due to even counter");
    }
}

/*
* _rootReverts = false
*
* EOA incrementAndCall(false) 
* 1 < 5 -> incrementAndCall(false)
* 2 < 5 -> incrementAndCall(false), REVERT
* 3 < 5 -> incrementAndCall(false)
* 4 < 5 -> incrementAndCall(false), REVERT
* 5 = 5 -> STOP
*/

/*
* _rootReverts = true
*
* EOA incrementAndCall(true) 
* 1 < 5 -> incrementAndCall(true), REVERT
* 2 < 5 -> incrementAndCall(true)
* 3 < 5 -> incrementAndCall(true), REVERT
* 4 < 5 -> incrementAndCall(true)
* 5 = 5 -> REVERT
*/



