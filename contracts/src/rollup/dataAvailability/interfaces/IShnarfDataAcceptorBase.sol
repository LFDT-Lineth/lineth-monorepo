// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

/**
 * @title Interface for shared dataRollingHash accepting functions, errors and events.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IShnarfDataAcceptorBase {
  /**
   * @dev Thrown when the dataRollingHash being submitted is the zero hash.
   */
  error DataRollingHashSubmissionIsZeroHash();

  /**
   * @dev Thrown when the current dataRollingHash was already anchored.
   */
  error DataRollingHashAlreadyAnchored(bytes32 dataRollingHash);

  /**
   * @dev Thrown when no anchored dataRollingHash exists for the parent position.
   */
  error ParentDataRollingHashNotAnchored(bytes32 dataRollingHash);

  /**
   * @dev Thrown when the computed dataRollingHash does not match what is expected.
   */
  error FinalDataRollingHashWrong(bytes32 expected, bytes32 value);

  /**
   * @notice Emitted when compressed data is being submitted and anchored successfully on L1.
   * @dev The parent dataRollingHash is included for state reconstruction simplicity.
   * @param parentDataRollingHash The parent dataRollingHash for the data being submitted.
   * @param dataRollingHash The indexed dataRollingHash anchored for the data being submitted.
   */
  event DataSubmittedV4(bytes32 parentDataRollingHash, bytes32 indexed dataRollingHash);
}
