// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { Validium } from "./Validium.sol";

/**
 * @title Temporary placeholder Validium contract mirroring Validium with no-op data submission and finalization.
 * @dev acceptDataRollingHash and finalizeBlocks are overridden as no-ops - all other behavior is inherited unchanged from Validium.
 * @author Consensys Incorporated
 * @custom:security-contact security-report@linea.build
 */
contract ValidiumV3Stub is Validium {
  /**
   * @notice No-op placeholder for accepting and anchoring a dataRollingHash.
   * @param _parentDataRollingHash The parent dataRollingHash.
   * @param _storedDataRollingHash The dataRollingHash to anchor.
   */
  function acceptDataRollingHash(
    bytes32 _parentDataRollingHash,
    bytes32 _storedDataRollingHash
  ) public override whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {}

  /**
   * @notice No-op placeholder for finalizing compressed blocks with proof.
   * @param _aggregatedProof The aggregated proof.
   * @param _proofType The proof type.
   * @param _finalizationData The full finalization data.
   */
  function finalizeBlocks(
    bytes calldata _aggregatedProof,
    uint256 _proofType,
    FinalizationDataV5 calldata _finalizationData
  ) external override whenTypeAndGeneralNotPaused(PauseType.FINALIZATION) onlyRole(OPERATOR_ROLE) {}
}
