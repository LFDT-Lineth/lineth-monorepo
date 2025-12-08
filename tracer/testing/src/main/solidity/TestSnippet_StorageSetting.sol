// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;

import {TestingFrameworkStorageLayout} from "./TestingFrameworkStorageLayout.sol";
import {TestingBase} from "./TestingBase.sol";

contract TestSnippet_StorageSetting is TestingFrameworkStorageLayout {
    // use 0xc35824a8000000000000000000000000000000000000000000000000000000000001e240 on the framework executeCalls to set `188992` as a test
    function setSecondInt(uint256 _val) public {
        secondInt = _val;
    }

    // use 0x2792dcc80000000000000000000000000000000000000000000000000db4da5f7ef412b1  setSecondIntBasedOnFirstInt(987654321987654321)
    function setSecondIntBasedOnFirstInt(uint256 _secondInt) public {
        if (firstInt == 0) {
            revert("notSet");
        }

        secondInt = _secondInt;
    }

    fallback() external payable {}

    receive() external payable {}
}
