// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

/**
 * @title Contract that helps prevent reentrant calls.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract TransientStorageReentrancyGuardUpgradeable {
  uint256 transient TRANSIENT_ENTERED;
  uint256 private constant NOT_ENTERED = 0;
  uint256 private constant ENTERED = 1;

  error ReentrantCall();

  /// @dev This gap is used to not shift down the storage layout after removing the OpenZeppelin ReentrancyGuardUpgradeable contract.
  uint256[50] private __gap_ReentrancyGuardUpgradeable;

  /**
   * @notice Checks reentrancy and if not reentrant sets the transient reentry flag.
   */
  modifier nonReentrant() {
    if (TRANSIENT_ENTERED == ENTERED) {
      revert ReentrantCall();
    }
    TRANSIENT_ENTERED = ENTERED;
    _;
    TRANSIENT_ENTERED = NOT_ENTERED;
  }
}
