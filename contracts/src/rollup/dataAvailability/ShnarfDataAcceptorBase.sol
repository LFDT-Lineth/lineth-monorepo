// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IShnarfDataAcceptorBase } from "./interfaces/IShnarfDataAcceptorBase.sol";
import { LinethRollupBase } from "../LinethRollupBase.sol";

/**
 * @title Contract to manage shared functions for dataRollingHash accepting and anchoring.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract ShnarfDataAcceptorBase is LinethRollupBase, IShnarfDataAcceptorBase {
  /// @dev Value indicating a dataRollingHash is anchored.
  uint256 internal constant DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE = 1;

  /**
   * @notice Accepts and anchors that a dataRollingHash exists.
   * @dev Anchoring only stores the final dataRollingHash of a submission; intermediate chunk
   *   folds are not persisted. A stream is continued across submissions by chaining from any
   *   previously-anchored parent dataRollingHash.
   * @param _parentDataRollingHash The parent dataRollingHash.
   * @param _dataRollingHash The dataRollingHash to anchor.
   */
  function _acceptShnarfData(bytes32 _parentDataRollingHash, bytes32 _dataRollingHash) internal virtual {
    require(
      _dataRollingHashExists[_parentDataRollingHash] != 0,
      ParentDataRollingHashNotAnchored(_parentDataRollingHash)
    );
    require(_dataRollingHashExists[_dataRollingHash] == 0, DataRollingHashAlreadyAnchored(_dataRollingHash));

    _dataRollingHashExists[_dataRollingHash] = DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE;

    emit DataSubmittedV4(_parentDataRollingHash, _dataRollingHash);
  }
}
