// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;
import { IShnarfDataAcceptorBase } from "./IShnarfDataAcceptorBase.sol";

/**
 * @title Interface for defining EIP-4844 blob submission functions, structs and errors.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAcceptEip4844Blobs is IShnarfDataAcceptorBase {
  /**
   * @dev Thrown when the blobhash at an index equals to the zero hash.
   */
  error EmptyBlobDataAtIndex(uint256 index);

  /**
   * @dev Thrown when the data for multiple blobs submission has length zero.
   */
  error BlobSubmissionDataIsMissing();

  /**
   * @dev Thrown when a blob has been submitted but there is no data for it.
   */
  error BlobSubmissionDataEmpty(uint256 emptyBlobIndex);

  /**
   * @notice Submit one or more EIP-4844 blobs.
   * @dev OPERATOR_ROLE is required to execute.
   * @dev This should be a blob carrying transaction.
   * @param _blobFinalBlockHashes The final L2 block hash for each blob being submitted.
   * @param _parentShnarf The parent shnarf used in continuity checks.
   * @param _finalBlobShnarf The expected final shnarf post computation of all the blob shnarfs.
   */
  function submitBlobs(
    bytes32[] calldata _blobFinalBlockHashes,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
  ) external;
}
