// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IAcceptCalldataBlobs } from "./interfaces/IAcceptCalldataBlobs.sol";
import { LocalShnarfProvider } from "./LocalShnarfProvider.sol";
import { EfficientLeftRightKeccak } from "../../libraries/EfficientLeftRightKeccak.sol";
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
   * @param _parentShnarf The parent shnarf used in continuity checks as it includes the parentStateRootHash in its computation.
   * @param _expectedShnarf The expected shnarf post computation of all the submission.
   */
  function submitDataAsCalldata(
    CompressedCalldataSubmission calldata _submission,
    bytes32 _parentShnarf,
    bytes32 _expectedShnarf
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.STATE_DATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    _submitDataAsCalldata(_submission, _parentShnarf, _expectedShnarf);
  }

  /**
   * @notice Submit blobs using compressed data via calldata.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _submission The supporting data for compressed data submission including compressed data.
   * @param _parentShnarf The parent shnarf used in continuity checks as it includes the parentStateRootHash in its computation.
   * @param _expectedShnarf The expected shnarf post computation of all the submission.
   */
  function _submitDataAsCalldata(
    CompressedCalldataSubmission calldata _submission,
    bytes32 _parentShnarf,
    bytes32 _expectedShnarf
  ) internal virtual {
    if (_submission.compressedData.length == 0) {
      revert EmptySubmissionData();
    }

    bytes32 currentDataHash = keccak256(_submission.compressedData);
    bytes32 dataEvaluationPoint = EfficientLeftRightKeccak._efficientKeccak(_submission.snarkHash, currentDataHash);
    bytes32 computedShnarf = _computeShnarf(
      _parentShnarf,
      _submission.snarkHash,
      _submission.finalStateRootHash,
      dataEvaluationPoint,
      _calculateY(_submission.compressedData, dataEvaluationPoint)
    );

    _acceptShnarfData(_parentShnarf, _expectedShnarf, _submission.finalStateRootHash);

    if (_expectedShnarf != computedShnarf) {
      revert FinalShnarfWrong(_expectedShnarf, computedShnarf);
    }
  }

  /**
   * @notice Internal function to calculate Y for public input generation.
   * @param _data Compressed data from submission data.
   * @param _dataEvaluationPoint The data evaluation point.
   * @dev Each chunk of 32 bytes must start with a 0 byte.
   * @dev The dataEvaluationPoint value is modulo-ed down during the computation and scalar field checking is not needed.
   * @dev There is a hard constraint in the circuit to enforce the polynomial degree limit (4096), which will also be enforced with EIP-4844.
   * @return compressedDataComputedY The Y calculated value using the Horner method.
   */
  function _calculateY(
    bytes calldata _data,
    bytes32 _dataEvaluationPoint
  ) internal pure returns (bytes32 compressedDataComputedY) {
    if (_data.length % 0x20 != 0) {
      revert BytesLengthNotMultipleOf32();
    }

    bytes4 errorSelector = IAcceptCalldataBlobs.FirstByteIsNotZero.selector;
    assembly {
      for {
        let i := _data.length
      } gt(i, 0) {} {
        i := sub(i, 0x20)
        let chunk := calldataload(add(_data.offset, i))
        if iszero(iszero(and(chunk, 0xFF00000000000000000000000000000000000000000000000000000000000000))) {
          let ptr := mload(0x40)
          mstore(ptr, errorSelector)
          revert(ptr, 0x4)
        }
        compressedDataComputedY := addmod(
          mulmod(compressedDataComputedY, _dataEvaluationPoint, BLS_CURVE_MODULUS),
          chunk,
          BLS_CURVE_MODULUS
        )
      }
    }
  }
}
