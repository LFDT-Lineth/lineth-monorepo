// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptEip4844Blobs } from "./interfaces/IAcceptEip4844Blobs.sol";
import { LocalShnarfProvider } from "./LocalShnarfProvider.sol";
import { EfficientLeftRightKeccak } from "../../libraries/EfficientLeftRightKeccak.sol";
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
   * @param _blobSubmissions The data for blob submission including proofs and required polynomials.
   * @param _parentShnarf The parent shnarf used in continuity checks as it includes the parentStateRootHash in its computation.
   * @param _finalBlobShnarf The expected final shnarf post computation of all the blob shnarfs.
   */
  function submitBlobs(
    BlobSubmission[] calldata _blobSubmissions,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
  ) external virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitBlobs(_blobSubmissions, _parentShnarf, _finalBlobShnarf);
  }

  /**
   * @notice Submit one or more EIP-4844 blobs.
   * @param _blobSubmissions The data for blob submission including proofs and required polynomials.
   * @param _parentShnarf The parent shnarf used in continuity checks as it includes the parentStateRootHash in its computation.
   * @param _finalBlobShnarf The expected final shnarf post computation of all the blob shnarfs.
   */
  function _submitBlobs(
    BlobSubmission[] calldata _blobSubmissions,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
  ) internal virtual {
    if (_blobSubmissions.length == 0) {
      revert BlobSubmissionDataIsMissing();
    }

    if (blobhash(_blobSubmissions.length) != EMPTY_HASH) {
      revert BlobSubmissionDataEmpty(_blobSubmissions.length);
    }

    bytes32 currentDataEvaluationPoint;
    bytes32 currentDataHash;

    /// @dev Assigning in memory saves a lot of gas vs. calldata reading.
    BlobSubmission memory blobSubmission;

    bytes32 computedShnarf = _parentShnarf;

    for (uint256 i; i < _blobSubmissions.length; i++) {
      blobSubmission = _blobSubmissions[i];

      currentDataHash = blobhash(i);

      if (currentDataHash == EMPTY_HASH) {
        revert EmptyBlobDataAtIndex(i);
      }

      bytes32 snarkHash = blobSubmission.snarkHash;

      currentDataEvaluationPoint = EfficientLeftRightKeccak._efficientKeccak(snarkHash, currentDataHash);

      _verifyPointEvaluation(
        currentDataHash,
        uint256(currentDataEvaluationPoint),
        blobSubmission.dataEvaluationClaim,
        blobSubmission.kzgCommitment,
        blobSubmission.kzgProof
      );

      computedShnarf = _computeShnarf(
        computedShnarf,
        snarkHash,
        blobSubmission.finalStateRootHash,
        currentDataEvaluationPoint,
        bytes32(blobSubmission.dataEvaluationClaim)
      );
    }

    _acceptShnarfData(_parentShnarf, _finalBlobShnarf, blobSubmission.finalStateRootHash);

    if (_finalBlobShnarf != computedShnarf) {
      revert FinalShnarfWrong(_finalBlobShnarf, computedShnarf);
    }
  }

  /**
   * @notice Performs point evaluation for the compressed blob.
   * @dev _dataEvaluationPoint is modular reduced to be lower than the BLS_CURVE_MODULUS for precompile checks.
   * @param _currentDataHash The current blob versioned hash.
   * @param _dataEvaluationPoint The data evaluation point.
   * @param _dataEvaluationClaim The data evaluation claim.
   * @param _kzgCommitment The blob KZG commitment.
   * @param _kzgProof The blob KZG point proof.
   */
  function _verifyPointEvaluation(
    bytes32 _currentDataHash,
    uint256 _dataEvaluationPoint,
    uint256 _dataEvaluationClaim,
    bytes memory _kzgCommitment,
    bytes memory _kzgProof
  ) internal view {
    assembly {
      _dataEvaluationPoint := mod(_dataEvaluationPoint, BLS_CURVE_MODULUS)
    }

    (bool success, bytes memory returnData) = POINT_EVALUATION_PRECOMPILE_ADDRESS.staticcall(
      abi.encodePacked(_currentDataHash, _dataEvaluationPoint, _dataEvaluationClaim, _kzgCommitment, _kzgProof)
    );

    if (!success) {
      revert PointEvaluationFailed();
    }

    if (returnData.length != POINT_EVALUATION_RETURN_DATA_LENGTH) {
      revert PrecompileReturnDataLengthWrong(POINT_EVALUATION_RETURN_DATA_LENGTH, returnData.length);
    }

    uint256 fieldElements;
    uint256 blsCurveModulus;
    assembly {
      fieldElements := mload(add(returnData, 0x20))
      blsCurveModulus := mload(add(returnData, POINT_EVALUATION_RETURN_DATA_LENGTH))
    }
    if (fieldElements != POINT_EVALUATION_FIELD_ELEMENTS_LENGTH || blsCurveModulus != BLS_CURVE_MODULUS) {
      revert PointEvaluationResponseInvalid(fieldElements, blsCurveModulus);
    }
  }
}
