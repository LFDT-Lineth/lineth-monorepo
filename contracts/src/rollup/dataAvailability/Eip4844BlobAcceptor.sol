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
   * @dev This should be a blob carrying transaction.
   * @param _blobFinalBlockHashes The final L2 block hash for each blob being submitted.
   * @param _parentShnarf The parent shnarf used in continuity checks.
   * @param _finalBlobShnarf The expected final shnarf post computation of all the blob shnarfs.
   */
  function submitBlobs(
    bytes32[] calldata _blobFinalBlockHashes,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitBlobs(_blobFinalBlockHashes, _parentShnarf, _finalBlobShnarf);
  }

  /**
   * @notice Internal implementation of EIP-4844 blob submission.
   * @param _blobFinalBlockHashes The final L2 block hash for each blob being submitted.
   * @param _parentShnarf The parent shnarf used in continuity checks.
   * @param _finalBlobShnarf The expected final shnarf post computation of all the blob shnarfs.
   */
  function _submitBlobs(
    bytes32[] calldata _blobFinalBlockHashes,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
  ) internal virtual {
    uint256 blobCount = _blobFinalBlockHashes.length;

    require(blobCount != 0, BlobSubmissionDataIsMissing());
    require(blobhash(blobCount) == EMPTY_HASH, BlobSubmissionDataEmpty(blobCount));

    bytes32 currentBlobHash;
    bytes32 computedShnarf = _parentShnarf;

    for (uint256 i; i < blobCount; i++) {
      currentBlobHash = blobhash(i);

      require(currentBlobHash != EMPTY_HASH, EmptyBlobDataAtIndex(i));
      require(_blobFinalBlockHashes[i] != EMPTY_HASH, FinalBlockHashIsZeroHash());

      computedShnarf = _computeShnarf(computedShnarf, _blobFinalBlockHashes[i], currentBlobHash);
    }

    require(_finalBlobShnarf == computedShnarf, FinalShnarfWrong(_finalBlobShnarf, computedShnarf));

    _acceptShnarfData(_parentShnarf, _finalBlobShnarf, _blobFinalBlockHashes[blobCount - 1]);
  }
}
