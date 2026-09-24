// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptDataRollingHash } from "./interfaces/IAcceptDataRollingHash.sol";
import { DataRollingHashAcceptorBase } from "./DataRollingHashAcceptorBase.sol";

/**
 * @title Contract to manage L2 dataRollingHash anchoring on L1 for rollup proof verification.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract DataRollingHashAcceptor is IAcceptDataRollingHash, DataRollingHashAcceptorBase {
  /**
   * @notice Accepts and anchors that a dataRollingHash exists.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _parentDataRollingHash The parent dataRollingHash.
   * @param _storedDataRollingHash The dataRollingHash to anchor.
   */
  function acceptDataRollingHash(
    bytes32 _parentDataRollingHash,
    bytes32 _storedDataRollingHash
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    require(_storedDataRollingHash != 0x0, DataRollingHashSubmissionIsZeroHash());
    _acceptDataRollingHash(_parentDataRollingHash, _storedDataRollingHash);
  }
}
