// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptShnarfData } from "./interfaces/IAcceptShnarfData.sol";
import { ShnarfDataAcceptorBase } from "./ShnarfDataAcceptorBase.sol";

/**
 * @title Contract to manage L2 shnarf data submission on L1 for rollup proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract ShnarfDataAcceptor is IAcceptShnarfData, ShnarfDataAcceptorBase {
  /**
   * @notice Accepts and stores that a shnarf exists.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _parentShnarf The parent shnarf.
   * @param _shnarf The shnarf to indicate exists.
   * @param _finalStateRootHash The final state root hash in the data.
   */
  function acceptShnarfData(
    bytes32 _parentShnarf,
    bytes32 _shnarf,
    bytes32 _finalStateRootHash
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    require(_shnarf != 0x0, ShnarfSubmissionIsZeroHash());
    require(_finalStateRootHash != 0x0, FinalStateRootHashIsZeroHash());
    _acceptShnarfData(_parentShnarf, _shnarf, _finalStateRootHash);
  }
}
