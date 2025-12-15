// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;

import {TestingFrameworkStorageLayout} from "./TestingFrameworkStorageLayout.sol";
import {TestingBase} from "./TestingBase.sol";

/**
 * @notice Contract to test self referencing and multi-level calls.
 */
contract SelfDestructingDeeperCalls is
    TestingFrameworkStorageLayout,
    TestingBase
{
    constructor() payable {}

    function setSelfReferencedCallExecuted() public {
        selfReferencedCallExecuted = true;
    }

    fallback() external payable {}

    receive() external payable {}
}
