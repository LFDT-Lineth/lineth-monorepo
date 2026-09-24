// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IDataRollingHashAcceptorBase } from "./interfaces/IDataRollingHashAcceptorBase.sol";
import { LinethRollupBase } from "../LinethRollupBase.sol";

/**
 * @title Contract to manage shared functions for dataRollingHash accepting and anchoring.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract DataRollingHashAcceptorBase is LinethRollupBase, IDataRollingHashAcceptorBase {
  /// @dev Value indicating a dataRollingHash is anchored.
  uint256 internal constant DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE = 1;

  /**
   * @notice Accepts and anchors that a dataRollingHash exists.
   * @dev Anchoring only stores the final dataRollingHash of a submission; intermediate chunk
   *   folds are not persisted. A stream is continued across submissions by chaining from any
   *   previously-anchored parent dataRollingHash.
   * @dev `currentFinalizedShnarf_DEPRECATED` is permanently treated as an anchored parent. For a
   *   genuine genesis fresh-start (or once the one-time legacy-shnarf migration has completed) it
   *   is EMPTY_HASH, matching prior behavior. While a network is still awaiting that migration it
   *   instead holds the bridged legacy shnarf value, so the first post-upgrade chunk explicitly
   *   chains from the pre-migration state rather than an arbitrary EMPTY_HASH reset.
   * @param _parentDataRollingHash The parent dataRollingHash.
   * @param _storedDataRollingHash The dataRollingHash to anchor.
   */
  function _acceptDataRollingHash(bytes32 _parentDataRollingHash, bytes32 _storedDataRollingHash) internal virtual {
    require(
      _dataRollingHashExists[_parentDataRollingHash] != 0 ||
        _parentDataRollingHash == currentFinalizedShnarf_DEPRECATED,
      ParentDataRollingHashNotAnchored(_parentDataRollingHash)
    );
    require(
      _dataRollingHashExists[_storedDataRollingHash] == 0,
      DataRollingHashAlreadyAnchored(_storedDataRollingHash)
    );

    _dataRollingHashExists[_storedDataRollingHash] = DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE;

    emit DataSubmittedV4(_parentDataRollingHash, _storedDataRollingHash);
  }
}
