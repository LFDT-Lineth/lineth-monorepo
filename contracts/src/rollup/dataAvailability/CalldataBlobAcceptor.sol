// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptCalldataBlobs } from "./interfaces/IAcceptCalldataBlobs.sol";
import { LocalShnarfProvider } from "./LocalShnarfProvider.sol";
import { ShnarfDataAcceptorBase } from "./ShnarfDataAcceptorBase.sol";

/**
 * @title Contract to manage compressed data blobs submitted as calldata.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract CalldataBlobAcceptor is LocalShnarfProvider, ShnarfDataAcceptorBase, IAcceptCalldataBlobs {
  /**
   * @notice Submit blobs using compressed data via calldata.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _submission The supporting data for compressed data submission including compressed data.
   * @param _parentShnarf The parent shnarf used in continuity checks.
   * @param _expectedShnarf The expected shnarf post computation of the submission.
   */
  function submitDataAsCalldata(
    CompressedCalldataSubmissionV2 calldata _submission,
    bytes32 _parentShnarf,
    bytes32 _expectedShnarf
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitDataAsCalldata(_submission, _parentShnarf, _expectedShnarf);
  }

  /**
   * @notice Internal implementation of calldata blob submission.
   * @param _submission The supporting data for compressed data submission including compressed data.
   * @param _parentShnarf The parent shnarf used in continuity checks.
   * @param _expectedShnarf The expected shnarf post computation of the submission.
   */
  function _submitDataAsCalldata(
    CompressedCalldataSubmissionV2 calldata _submission,
    bytes32 _parentShnarf,
    bytes32 _expectedShnarf
  ) internal virtual {
    require(_submission.compressedData.length != 0, EmptySubmissionData());
    require(_submission.blockHash != 0x0, FinalBlockHashIsZeroHash());

    bytes32 dataHash = keccak256(_submission.compressedData);
    bytes32 computedShnarf = _computeShnarf(_parentShnarf, _submission.blockHash, dataHash);

    require(_expectedShnarf == computedShnarf, FinalShnarfWrong(_expectedShnarf, computedShnarf));

    _acceptShnarfData(_parentShnarf, _expectedShnarf, _submission.blockHash);
  }
}
