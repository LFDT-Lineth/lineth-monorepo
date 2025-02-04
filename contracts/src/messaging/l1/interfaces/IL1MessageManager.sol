// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.26;

/**
 * @title L1 Message manager interface for current functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IL1MessageManager {
  /**
   * @notice Emitted when a new message is sent and the rolling hash updated.
   * @param messageNumber The unique indexed message number for the message.
   * @param rollingHash The indexed rolling hash computed for the current message number.
   * @param messageHash The indexed hash of the message parameters.
   */
  event RollingHashUpdated(uint256 indexed messageNumber, bytes32 indexed rollingHash, bytes32 indexed messageHash);

  /**
   * @notice Emitted when the L2 Merkle root has been anchored on L1.
   * @param l2MerkleRoot The indexed L2 Merkle root that has been anchored on L1 Ethereum.
   * @param treeDepth The indexed tree depth of the Merkle root.
   * @dev There may be more than one of these in a finalization depending on the amount of L2->L1 messages in the finalization.
   */
  event L2MerkleRootAdded(bytes32 indexed l2MerkleRoot, uint256 indexed treeDepth);

  /**
   * @notice Emitted when the L2 block contains L2 messages during finalization.
   * @param l2Block The indexed L2 block containing L2 to L1 messages.
   * @dev This is used externally in the logic for determining which messages belong to which Merkle root when claiming.
   */
  event L2MessagingBlockAnchored(uint256 indexed l2Block);

  /**
   * @dev Thrown when the message has already been claimed.
   */
  error MessageAlreadyClaimed(uint256 messageIndex);

  /**
   * @dev Thrown when the L2 Merkle root has already been anchored on L1.
   */
  error L2MerkleRootAlreadyAnchored(bytes32 merkleRoot);

  /**
   * @dev Thrown when the L2 messaging blocks offsets bytes length is not a multiple of 2.
   */
  error BytesLengthNotMultipleOfTwo(uint256 bytesLength);

  /**
   * @notice Checks if the L2->L1 message is claimed or not.
   * @param _messageNumber The message number on L2.
   * @return isClaimed Returns whether or not the message with _messageNumber has been claimed.
   */
  function isMessageClaimed(uint256 _messageNumber) external view returns (bool isClaimed);
}
