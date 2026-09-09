// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptCalldataBlobs } from "./interfaces/IAcceptCalldataBlobs.sol";
import { LocalShnarfProvider } from "./LocalShnarfProvider.sol";
import { ShnarfDataAcceptorBase } from "./ShnarfDataAcceptorBase.sol";

/**
 * @title Contract to manage compressed data chunks submitted as calldata.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract CalldataBlobAcceptor is LocalShnarfProvider, ShnarfDataAcceptorBase, IAcceptCalldataBlobs {
  /**
   * @notice Submit a compressed data chunk via calldata.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _compressedData The compressed transaction data for the chunk being submitted.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _expectedDataRollingHash The expected dataRollingHash after folding this chunk.
   */
  function submitDataAsCalldata(
    bytes calldata _compressedData,
    bytes32 _parentDataRollingHash,
    bytes32 _expectedDataRollingHash
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitDataAsCalldata(_compressedData, _parentDataRollingHash, _expectedDataRollingHash);
  }

  /**
   * @notice Internal implementation of calldata chunk submission.
   * @dev The chunk hash is keccak256(_compressedData), folded once into the dataRollingHash
   *   accumulator. Execution continuity is not tied to submission (no per-chunk block hash).
   * @param _compressedData The compressed transaction data for the chunk being submitted.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _expectedDataRollingHash The expected dataRollingHash after folding this chunk.
   */
  function _submitDataAsCalldata(
    bytes calldata _compressedData,
    bytes32 _parentDataRollingHash,
    bytes32 _expectedDataRollingHash
  ) internal virtual {
    require(_compressedData.length != 0, EmptySubmissionData());

    bytes32 chunkHash = keccak256(_compressedData);
    bytes32 computedDataRollingHash = _computeDataRollingHash(_parentDataRollingHash, chunkHash);

    require(
      _expectedDataRollingHash == computedDataRollingHash,
      FinalDataRollingHashWrong(_expectedDataRollingHash, computedDataRollingHash)
    );

    _acceptShnarfData(_parentDataRollingHash, _expectedDataRollingHash);
  }
}
