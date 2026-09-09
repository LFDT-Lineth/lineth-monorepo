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
   * @dev Thrown when a blob carrying transaction carries no blobs.
   */
  error BlobSubmissionDataIsMissing();

  /**
   * @notice Submit one or more EIP-4844 blobs.
   * @dev OPERATOR_ROLE is required to execute.
   * @dev This should be a blob carrying transaction. Each carried blob's versioned hash
   *   (via the EIP-4844 `blobhash` opcode) is folded into the dataRollingHash accumulator.
   *   No per-blob calldata is supplied: chunk boundaries carry no block/conflation semantics.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _finalDataRollingHash The expected final dataRollingHash after folding all blobs.
   */
  function submitBlobs(bytes32 _parentDataRollingHash, bytes32 _finalDataRollingHash) external;
}
