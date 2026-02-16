// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

/**
 * @title Contract that helps prevent reentrant calls.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract TransientStorageReentrancyGuardUpgradeable {
  /// @dev The key in the EVM transient storage where the reentrancy guard is stored.
  // bytes32(uint256(keccak256("eip1967.reentrancy.guard.transient.key")) - 1);
  bytes32 private constant REENTRANCY_GUARD_TRANSIENT_KEY =
    0x084edf88d5959696dcc7aab5c8674a33a1ef78f37dda21b782ed03bddb22ade4;
  uint256 private constant NOT_ENTERED = 0;
  uint256 private constant ENTERED = 1;

  /// @dev The error that is thrown when a reentrant call is detected.
  error ReentrantCall();

  /// @dev This gap is used to not shift down the storage layout after removing the OpenZeppelin ReentrancyGuardUpgradeable contract.
  uint256[50] private __gap_ReentrancyGuardUpgradeable;

  /**
   * @notice Checks reentrancy and if not reentrant sets the transient reentry flag.
   * @dev The selector for the ReentrantCall error.
   * ReentrantCall.selector = 0x37ed32e8
   */
  modifier nonReentrant() {
    assembly ("memory-safe") {
      if eq(tload(REENTRANCY_GUARD_TRANSIENT_KEY), ENTERED) {
        mstore(0x00, 0x37ed32e8)
        revert(0x1c, 0x04)
      }

      tstore(REENTRANCY_GUARD_TRANSIENT_KEY, ENTERED)
    }

    _;
    assembly ("memory-safe") {
      tstore(REENTRANCY_GUARD_TRANSIENT_KEY, NOT_ENTERED)
    }
  }
}
