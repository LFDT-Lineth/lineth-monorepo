// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IShnarfDataAcceptorBase } from "./IShnarfDataAcceptorBase.sol";

/**
 * @title Interface for defining calldata blob submission functions, structs and errors.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAcceptCalldataBlobs is IShnarfDataAcceptorBase {
  /**
   * @notice Supporting data for compressed calldata submission including compressed data.
   * @param blockHash The L2 final block hash for this submission, used in shnarf computation.
   * @param compressedData is the compressed transaction data. It contains ordered data for each L2 block - l2Timestamps, the encoded transaction data.
   */
  struct CompressedCalldataSubmissionV2 {
    bytes32 blockHash;
    bytes compressedData;
  }

  /**
   * @dev Thrown when submissionData is empty.
   */
  error EmptySubmissionData();

  /**
   * @notice Submit blobs using compressed data via calldata.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _submission The supporting data for compressed data submission including compressed data.
   * @param _parentShnarf The parent shnarf used in continuity checks.
   * @param _expectedShnarf The expected shnarf post computation of all the submission.
   */
  function submitDataAsCalldata(
    CompressedCalldataSubmissionV2 calldata _submission,
    bytes32 _parentShnarf,
    bytes32 _expectedShnarf
  ) external;
}
