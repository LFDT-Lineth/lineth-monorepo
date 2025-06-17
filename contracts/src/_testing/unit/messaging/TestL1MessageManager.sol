// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { L1MessageManager } from "../../../messaging/l1/L1MessageManager.sol";

contract TestL1MessageManager is L1MessageManager {
  /**
   * @dev Thrown when the L1->L2 message has not been sent.
   */
  error L1L2MessageNotSent(bytes32 messageHash);

  /**
   * @dev Thrown when the message has already been received.
   */
  error MessageAlreadyReceived(bytes32 messageHash);

  ///@dev V1
  function addL2L1MessageHash(bytes32 _messageHash) external {
    if (inboxL2L1MessageStatus[_messageHash] != INBOX_STATUS_UNKNOWN) {
      revert MessageAlreadyReceived(_messageHash);
    }

    inboxL2L1MessageStatus[_messageHash] = INBOX_STATUS_RECEIVED;
  }

  function updateL2L1MessageStatusToClaimed(bytes32 _messageHash) external {
    _updateL2L1MessageStatusToClaimed(_messageHash);
  }

  ///@dev V2
  function setL2L1MessageToClaimed(uint256 _messageNumber) external {
    _setL2L1MessageToClaimed(_messageNumber);
  }

  function addL2MerkleRoots(bytes32[] calldata _newRoot, uint256 _treeDepth) external {
    _addL2MerkleRoots(_newRoot, _treeDepth);
  }

  function anchorL2MessagingBlocks(bytes calldata _l2MessagingBlocksOffsets, uint256 _currentL2BlockNumber) external {
    _anchorL2MessagingBlocks(_l2MessagingBlocksOffsets, _currentL2BlockNumber);
  }
}
