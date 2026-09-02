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
  uint256 internal constant SHNARF_EXISTS_DEFAULT_VALUE = 1;

  /**
   * @notice Accepts and anchors that a dataRollingHash exists.
   * @dev Anchoring only stores the final dataRollingHash of a submission; intermediate chunk
   *   folds are not persisted. A stream is continued across submissions by chaining from any
   *   previously-anchored parent dataRollingHash.
   * @param _parentDataRollingHash The parent dataRollingHash.
   * @param _dataRollingHash The dataRollingHash to anchor.
   */
  function _acceptShnarfData(bytes32 _parentDataRollingHash, bytes32 _dataRollingHash) internal virtual {
    require(_blobShnarfExists[_parentDataRollingHash] != 0, ParentDataRollingHashNotAnchored(_parentDataRollingHash));
    require(_blobShnarfExists[_dataRollingHash] == 0, DataRollingHashAlreadyAnchored(_dataRollingHash));

    _blobShnarfExists[_dataRollingHash] = SHNARF_EXISTS_DEFAULT_VALUE;

    emit DataSubmittedV4(_parentDataRollingHash, _dataRollingHash);
  }
}
