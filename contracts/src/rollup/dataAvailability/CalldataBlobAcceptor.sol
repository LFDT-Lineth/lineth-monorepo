// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptCalldataBlobs } from "./interfaces/IAcceptCalldataBlobs.sol";
import { LocalDataRollingHashProvider } from "./LocalDataRollingHashProvider.sol";
import { DataRollingHashAcceptorBase } from "./DataRollingHashAcceptorBase.sol";

/**
 * @title Contract to manage compressed data chunks submitted as calldata.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract CalldataBlobAcceptor is
  LocalDataRollingHashProvider,
  DataRollingHashAcceptorBase,
  IAcceptCalldataBlobs
{
  /**
   * @notice Submit a compressed data chunk via calldata.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _compressedData The compressed transaction data for the chunk being submitted.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _storedDataRollingHash The dataRollingHash to store after folding this chunk.
   */
  function submitDataAsCalldata(
    bytes calldata _compressedData,
    bytes32 _parentDataRollingHash,
    bytes32 _storedDataRollingHash
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitDataAsCalldata(_compressedData, _parentDataRollingHash, _storedDataRollingHash);
  }

  /**
   * @notice Internal implementation of calldata chunk submission.
   * @dev The chunk hash is keccak256(_compressedData), folded once into the dataRollingHash
   *   accumulator. Execution continuity is not tied to submission (no per-chunk block hash).
   * @param _compressedData The compressed transaction data for the chunk being submitted.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _storedDataRollingHash The dataRollingHash to store after folding this chunk.
   */
  function _submitDataAsCalldata(
    bytes calldata _compressedData,
    bytes32 _parentDataRollingHash,
    bytes32 _storedDataRollingHash
  ) internal virtual {
    require(_compressedData.length != 0, EmptySubmissionData());

    bytes32 computedDataRollingHash = _computeDataRollingHash(_parentDataRollingHash, keccak256(_compressedData));

    require(
      _storedDataRollingHash == computedDataRollingHash,
      DataRollingHashMismatch(_storedDataRollingHash, computedDataRollingHash)
    );

    _acceptDataRollingHash(_parentDataRollingHash, _storedDataRollingHash);
  }
}
