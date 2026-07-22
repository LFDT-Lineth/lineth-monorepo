// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LineaRollup } from "./LineaRollup.sol";

/**
 * @title Temporary placeholder rollup contract mirroring LineaRollup with no-op data submission and finalization.
 * @dev submitBlobs and finalizeBlocks are overridden as no-ops - all other behavior is inherited unchanged from LineaRollup.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract LinethRollupV9Stub is LineaRollup {
  /**
   * @notice No-op placeholder for EIP-4844 blob submission.
   * @param _blobFinalBlockHashes The final L2 block hash for each blob being submitted.
   * @param _parentShnarf The parent shnarf used in continuity checks.
   * @param _finalBlobShnarf The expected final shnarf post computation of all the blob shnarfs.
   */
  function submitBlobs(
    bytes32[] calldata _blobFinalBlockHashes,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
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
