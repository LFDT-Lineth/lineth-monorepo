// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptEip4844Blobs } from "./interfaces/IAcceptEip4844Blobs.sol";
import { EfficientLeftRightKeccak } from "../../libraries/EfficientLeftRightKeccak.sol";
import { LocalDataRollingHashProvider } from "./LocalDataRollingHashProvider.sol";
import { DataRollingHashAcceptorBase } from "./DataRollingHashAcceptorBase.sol";

/**
 * @title Contract to manage EIP-4844 blob submission.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract Eip4844BlobAcceptor is
  LocalDataRollingHashProvider,
  DataRollingHashAcceptorBase,
  IAcceptEip4844Blobs
{
  /**
   * @notice Submit one or more EIP-4844 blobs.
   * @dev OPERATOR_ROLE is required to execute.
   * @dev This should be a blob carrying transaction. Each carried blob's versioned hash
   *   (via the EIP-4844 `blobhash` opcode) is folded into the dataRollingHash accumulator.
   *   Chunk boundaries carry no block/conflation semantics, so no per-blob calldata is supplied.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _storedDataRollingHash The dataRollingHash to store after folding all blobs.
   */
  function submitBlobs(
    bytes32 _parentDataRollingHash,
    bytes32 _storedDataRollingHash
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitBlobs(_parentDataRollingHash, _storedDataRollingHash);
  }

  /**
   * @notice Internal implementation of EIP-4844 blob submission.
   * @dev Folds blobhash(i) for every blob carried by the transaction into the running
   *   dataRollingHash, then anchors the final value. Only the final dataRollingHash of the
   *   submission is persisted; a stream is continued across submissions by chaining from any
   *   previously-anchored parent dataRollingHash.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _storedDataRollingHash The dataRollingHash to store after folding all blobs.
   */
  function _submitBlobs(bytes32 _parentDataRollingHash, bytes32 _storedDataRollingHash) internal virtual {
    require(blobhash(0) != EMPTY_HASH, BlobSubmissionDataIsMissing());

    bytes32 computedDataRollingHash = _parentDataRollingHash;
    bytes32 currentBlobHash;
    unchecked {
      for (uint256 i; ; i++) {
        currentBlobHash = blobhash(i);

        if (currentBlobHash == EMPTY_HASH) {
          break;
        }

        computedDataRollingHash = EfficientLeftRightKeccak._efficientKeccak(computedDataRollingHash, currentBlobHash);
      }
    }

    require(
      _storedDataRollingHash == computedDataRollingHash,
      DataRollingHashMismatch(_storedDataRollingHash, computedDataRollingHash)
    );

    _acceptDataRollingHash(_parentDataRollingHash, _storedDataRollingHash);
  }
}
