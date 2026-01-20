// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

/**
 * @title Interface for shared shnarf related data accepting functions, errors and events.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IShnarfDataAcceptorBase {
  /**
   * @dev Thrown when the shnarf being submitted is the zero hash.
   */
  error ShnarfSubmissionIsZeroHash();

  /**
   * @dev Thrown when the final state root hash being submitted is the zero hash.
   */
  error FinalStateRootHashIsZeroHash();

  /**
   * @dev Thrown when the current shnarf was already submitted.
   */
  error ShnarfAlreadySubmitted(bytes32 shnarf);

  /**
   * @dev Thrown when a shnarf does not exist for a parent blob.
   */
  error ParentShnarfNotSubmitted(bytes32 shnarf);

  /**
   * @dev Thrown when the computed shnarf does not match what is expected.
   */
  error FinalShnarfWrong(bytes32 expected, bytes32 value);

  /**
   * @notice Emitted when compressed data is being submitted and verified succesfully on L1.
   * @dev The block range is indexed and parent shnarf included for state reconstruction simplicity.
   * @param parentShnarf The parent shnarf for the data being submitted.
   * @param shnarf The indexed shnarf for the data being submitted.
   * @param finalStateRootHash The L2 state root hash that the current blob submission ends on. NB: The last blob in the collection.
   */
  event DataSubmittedV3(bytes32 parentShnarf, bytes32 indexed shnarf, bytes32 finalStateRootHash);
}
