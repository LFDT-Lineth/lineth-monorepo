// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.26;

import { TransientStorageHelpers } from "../../libraries/TransientStorageHelpers.sol";

/**
 * @title Contract that helps prevent reentrant calls.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract TransientStorageReentrancyGuardUpgradeable {
  using TransientStorageHelpers for *;

  bytes32 private constant REENTRANCY_GUARD_TRANSIENT_KEY =
    bytes32(uint256(keccak256("eip1967.reentrancy.guard.transient.key")) - 1);

  uint256 private constant NOT_ENTERED = 0;
  uint256 private constant ENTERED = 1;

  error ReentrantCall();

  /// @dev This gap is used to not shift down the storage layout after removing the OpenZeppelin ReentrancyGuardUpgradeable contract.
  uint256[50] private __gap_ReentrancyGuardUpgradeable;

  modifier nonReentrant() {
    _nonReentrantBefore();
    _;
    _nonReentrantAfter();
  }

  /**
   * @notice Checks reentrancy and if not reentrant sets the transient reentry flag.
   * @dev This uses the TransientStorageHelpers library and REENTRANCY_GUARD_TRANSIENT_KEY.
   */
  function _nonReentrantBefore() private {
    if (TransientStorageHelpers.tloadUint256(REENTRANCY_GUARD_TRANSIENT_KEY) != NOT_ENTERED) {
      revert ReentrantCall();
    }

    TransientStorageHelpers.tstoreUint256(REENTRANCY_GUARD_TRANSIENT_KEY, ENTERED);
  }

  /**
   * @notice Clears reentry transient storage flag.
   * @dev This uses the TransientStorageHelpers library and REENTRANCY_GUARD_TRANSIENT_KEY.
   */
  function _nonReentrantAfter() private {
    TransientStorageHelpers.tstoreUint256(REENTRANCY_GUARD_TRANSIENT_KEY, NOT_ENTERED);
  }
}
