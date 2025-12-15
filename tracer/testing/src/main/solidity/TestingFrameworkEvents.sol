// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;

/**
 * @notice Shared contract containing framework events.
 */
abstract contract TestingFrameworkEvents {
    event NoData();
    event DataNoIndexes(uint256 singleInt);
    event OneIndex(uint256 indexed singleInt);
    event TwoIndexes(uint256 indexed firstInt, uint256 indexed secondInt);
    event ThreeIndexes(
        uint256 indexed firstInt,
        uint256 indexed secondInt,
        uint256 indexed thirdInt
    );
}
