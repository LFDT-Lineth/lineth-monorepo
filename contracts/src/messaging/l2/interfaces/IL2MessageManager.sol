// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title Interface declaring cross-chain messaging on L2 functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IL2MessageManager {
  /**
   * @notice Emitted after all messages are anchored on L2 and the latest message index and rolling hash stored.
   * @param messageNumber The indexed unique L1 computed indexed message number for the message.
   * @param rollingHash The indexed L1 rolling hash computed for the current message number.
   * @dev NB: This event is used to provide data to the rollup. The last messageNumber and rollingHash,
   * emitted in a rollup will be used in the public input for validating the L1->L2 messaging state transition.
   */
  event RollingHashUpdated(uint256 indexed messageNumber, bytes32 indexed rollingHash);

  /**
   * @dev Emitted when the service switches over to a new version.
   * @dev This is currently not in use, but left for existing consumers.
   * @param version The indexed version.
   */
  event ServiceVersionMigrated(uint256 indexed version);

  /**
   * @dev Reverts when the message hashes array length is zero.
   */
  error MessageHashesListLengthIsZero();

  /**
   * @dev Reverts when message number synchronization is mismatched.
   */
  error L1MessageNumberSynchronizationWrong(uint256 expected, uint256 found);

  /**
   * @dev Reverts when rolling hash synchronization is mismatched.
   */
  error L1RollingHashSynchronizationWrong(bytes32 expected, bytes32 found);

  /**
   * @dev Reverts when final rolling hash is zero hash.
   */
  error FinalRollingHashIsZero();

  /**
   * @notice Add cross-chain L1->L2 message hashes in storage.
   * @dev Only address that has the role 'L1_L2_MESSAGE_SETTER_ROLE' are allowed to call this function.
   * @dev NB: In the unlikely event of a duplicate anchoring, the lastAnchoredL1MessageNumber MUST NOT be incremented.
   * @dev and the rolling hash not calculated, else synchronisation will break.
   * @dev If starting number is zero, an underflow error is expected.
   * @param _messageHashes New message hashes to anchor on L2.
   * @param _startingMessageNumber The expected L1 message number to start when anchoring.
   * @param _finalMessageNumber The expected L1 message number to end on when anchoring.
   * @param _finalRollingHash The expected L1 rolling hash to end on when anchoring.
   */
  function anchorL1L2MessageHashes(
    bytes32[] calldata _messageHashes,
    uint256 _startingMessageNumber,
    uint256 _finalMessageNumber,
    bytes32 _finalRollingHash
  ) external;
}
