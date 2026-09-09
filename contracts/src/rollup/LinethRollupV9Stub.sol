// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LinethRollup } from "./LinethRollup.sol";

/**
 * @title Temporary placeholder rollup contract mirroring LinethRollup with no-op data submission and finalization.
 * @dev submitBlobs and finalizeBlocks are overridden as no-ops - all other behavior is inherited unchanged from LinethRollup.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract LinethRollupV9Stub is LinethRollup {
  /**
   * @notice No-op placeholder for EIP-4844 blob submission.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _finalDataRollingHash The expected final dataRollingHash after folding all blobs.
   */
  function submitBlobs(
    bytes32 _parentDataRollingHash,
    bytes32 _finalDataRollingHash
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
    FinalizationDataV6 calldata _finalizationData
  ) external override whenTypeAndGeneralNotPaused(PauseType.FINALIZATION) onlyRole(OPERATOR_ROLE) {}
}
