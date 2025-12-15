// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;

import {TestingFrameworkEvents} from "./TestingFrameworkEvents.sol";

/**
 * @notice Event emitting contract for purely testing events.
 */
contract TestSnippet_Events is TestingFrameworkEvents {
    // 0xb0bc1f76 emitNoData()
    function emitNoData() public {
        emit NoData();
    }

    // 0xbc5b9381000000000000000000000000000000000000000000000000000000000001e240 emitDataNoIndexes(123456)
    function emitDataNoIndexes(uint256 _singleInt) public {
        emit DataNoIndexes(_singleInt);
    }

    // 0xbd6f0b2f000000000000000000000000000000000000000000000000000000002f091180 emitOneIndex(789123456)
    function emitOneIndex(uint256 singleInt) public {
        emit OneIndex(singleInt);
    }

    // 0x0f1087cc000000000000000000000000000000000000000000000000000000002f09118000000000000000000000000000000000000000000000000000000000040bf02d emitTwoIndexes(789123456,67891245)
    function emitTwoIndexes(uint256 _firstInt, uint256 _secondInt) public {
        emit TwoIndexes(_firstInt, _secondInt);
    }

    // 0x4497a35e000000000000000000000000000000000000000000000000000000002f09118000000000000000000000000000000000000000000000000000000000040bf02d000000000000000000000000000000000000000000000000000000003ade68b1 emitThreeIndexes(789123456,67891245,987654321)
    function emitThreeIndexes(
        uint256 _firstInt,
        uint256 _secondInt,
        uint256 _thirdInt
    ) public {
        emit ThreeIndexes(_firstInt, _secondInt, _thirdInt);
    }

    fallback() external payable {}

    receive() external payable {}
}
