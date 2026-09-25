// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IDataRollingHashAcceptorBase } from "./IDataRollingHashAcceptorBase.sol";

/**
 * @title Interface for defining calldata blob submission functions, structs and errors.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAcceptCalldataBlobs is IDataRollingHashAcceptorBase {
  /**
   * @dev Thrown when submissionData is empty.
   */
  error EmptySubmissionData();

  /**
   * @notice Submit a compressed data chunk via calldata.
   * @dev OPERATOR_ROLE is required to execute.
   * @dev The chunk hash binding the data is keccak256(_compressedData), folded into the
   *   dataRollingHash accumulator. No per-submission block hash is carried: execution
   *   continuity is a rollup-proof public-input field, not tied to submission.
   * @param _compressedData The compressed transaction data for the chunk being submitted.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _storedDataRollingHash The dataRollingHash to store after folding this chunk.
   */
  function submitDataAsCalldata(
    bytes calldata _compressedData,
    bytes32 _parentDataRollingHash,
    bytes32 _storedDataRollingHash
  ) external;
}
