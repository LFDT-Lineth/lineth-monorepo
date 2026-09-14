// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptShnarfData } from "./interfaces/IAcceptShnarfData.sol";
import { ShnarfDataAcceptorBase } from "./ShnarfDataAcceptorBase.sol";

/**
 * @title Contract to manage L2 dataRollingHash anchoring on L1 for rollup proof verification.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract ShnarfDataAcceptor is IAcceptShnarfData, ShnarfDataAcceptorBase {
  /**
   * @notice Accepts and anchors that a dataRollingHash exists.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _parentDataRollingHash The parent dataRollingHash.
   * @param _dataRollingHash The dataRollingHash to anchor.
   */
  function acceptShnarfData(
    bytes32 _parentDataRollingHash,
    bytes32 _dataRollingHash
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    require(_dataRollingHash != 0x0, DataRollingHashSubmissionIsZeroHash());
    _acceptShnarfData(_parentDataRollingHash, _dataRollingHash);
  }
}
