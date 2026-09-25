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
   * @dev Every valid parent is anchored directly in `_dataRollingHashExists`: the deterministic
   *   genesis hash seeded at `__LinethRollup_init` for fresh networks, or the migrated legacy
   *   shnarf seeded at `reinitializeLineaRollupV10` for in-place upgrades. No special-case
   *   fallback is needed.
   * @param _parentDataRollingHash The parent dataRollingHash.
   * @param _storedDataRollingHash The dataRollingHash to anchor.
   */
  function _acceptDataRollingHash(bytes32 _parentDataRollingHash, bytes32 _storedDataRollingHash) internal virtual {
    require(
      _dataRollingHashExists[_parentDataRollingHash] != 0,
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
