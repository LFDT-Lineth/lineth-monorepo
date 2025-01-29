// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

/**
 * @title Contract to fill space in storage to maintain storage layout.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract StorageFiller39 {
  /// @dev Keep free storage slots for future implementation updates to avoid storage collision.
  uint256[39] private __gap_39;
}
