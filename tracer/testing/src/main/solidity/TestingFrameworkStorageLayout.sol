// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;

/**
 * @notice Shared storage layout for all contracts being delegated to.
 * @dev Customize this to your liking for scenarios.
 */
abstract contract TestingFrameworkStorageLayout {
    uint256 public firstInt;
    uint256 public secondInt;
    bool public selfReferencedCallExecuted;
    address public createdContract;
}
