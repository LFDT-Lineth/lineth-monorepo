// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptEip4844Blobs } from "./interfaces/IAcceptEip4844Blobs.sol";
import { LocalShnarfProvider } from "./LocalShnarfProvider.sol";
import { ShnarfDataAcceptorBase } from "./ShnarfDataAcceptorBase.sol";

/**
 * @title Contract to manage EIP-4844 blob submission.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract Eip4844BlobAcceptor is LocalShnarfProvider, ShnarfDataAcceptorBase, IAcceptEip4844Blobs {
  /**
   * @notice Submit one or more EIP-4844 blobs.
   * @dev OPERATOR_ROLE is required to execute.
   * @dev This should be a blob carrying transaction. Each carried blob's versioned hash
   *   (via the EIP-4844 `blobhash` opcode) is folded into the dataRollingHash accumulator.
   *   Chunk boundaries carry no block/conflation semantics, so no per-blob calldata is supplied.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _finalDataRollingHash The expected final dataRollingHash after folding all blobs.
   */
  function submitBlobs(
    bytes32 _parentDataRollingHash,
    bytes32 _finalDataRollingHash
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitBlobs(_parentDataRollingHash, _finalDataRollingHash);
  }

  /**
   * @notice Internal implementation of EIP-4844 blob submission.
   * @dev Folds blobhash(i) for every blob carried by the transaction into the running
   *   dataRollingHash, then anchors the final value. Only the final dataRollingHash of the
   *   submission is persisted; a stream is continued across submissions by chaining from any
   *   previously-anchored parent dataRollingHash.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _finalDataRollingHash The expected final dataRollingHash after folding all blobs.
   */
  function _submitBlobs(bytes32 _parentDataRollingHash, bytes32 _finalDataRollingHash) internal virtual {
    require(blobhash(0) != EMPTY_HASH, BlobSubmissionDataIsMissing());

    bytes32 computedDataRollingHash = _parentDataRollingHash;

    for (uint256 i; ; i++) {
      bytes32 currentBlobHash = blobhash(i);

      if (currentBlobHash == EMPTY_HASH) {
        break;
      }

      computedDataRollingHash = _computeDataRollingHash(computedDataRollingHash, currentBlobHash);
    }

    require(
      _finalDataRollingHash == computedDataRollingHash,
      FinalDataRollingHashWrong(_finalDataRollingHash, computedDataRollingHash)
    );

    _acceptShnarfData(_parentDataRollingHash, _finalDataRollingHash);
  }
}
