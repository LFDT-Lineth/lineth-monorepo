// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L2MessageManagerV1 } from "./v1/L2MessageManagerV1.sol";
import { IL2MessageManager } from "./interfaces/IL2MessageManager.sol";
import { EfficientLeftRightKeccak } from "../../libraries/EfficientLeftRightKeccak.sol";

/**
 * @title Contract to manage cross-chain message hashes storage and statuses on L2.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract L2MessageManager is AccessControlUpgradeable, IL2MessageManager, L2MessageManagerV1 {
  using EfficientLeftRightKeccak for *;

  /// @notice The role required to anchor L1 to L2 message hashes.
  bytes32 public constant L1_L2_MESSAGE_SETTER_ROLE = keccak256("L1_L2_MESSAGE_SETTER_ROLE");

  /// @notice Contains the last L1 message number anchored on L2.
  uint256 public lastAnchoredL1MessageNumber;

  /// @notice Contains the L1 to L2 messaging rolling hashes mapped to message number computed on L2.
  mapping(uint256 messageNumber => bytes32 rollingHash) public l1RollingHashes;

  /// @dev Total contract storage is 52 slots including the gap below.
  /// @dev Keep 50 free storage slots for future implementation updates to avoid storage collision.
  uint256[50] private __gap_L2MessageManager;

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
  ) external whenTypeNotPaused(PauseType.GENERAL) onlyRole(L1_L2_MESSAGE_SETTER_ROLE) {
    if (_messageHashes.length == 0) {
      revert MessageHashesListLengthIsZero();
    }

    if (_messageHashes.length > 100) {
      revert MessageHashesListLengthHigherThanOneHundred(_messageHashes.length);
    }

    if (_finalRollingHash == 0x0) {
      revert FinalRollingHashIsZero();
    }

    uint256 currentL1MessageNumber = lastAnchoredL1MessageNumber;

    if (_startingMessageNumber - 1 != currentL1MessageNumber) {
      revert L1MessageNumberSynchronizationWrong(_startingMessageNumber - 1, currentL1MessageNumber);
    }

    bytes32 rollingHash = l1RollingHashes[currentL1MessageNumber];

    bytes32 messageHash;
    for (uint256 i; i < _messageHashes.length; ++i) {
      messageHash = _messageHashes[i];
      if (inboxL1L2MessageStatus[messageHash] == INBOX_STATUS_UNKNOWN) {
        inboxL1L2MessageStatus[messageHash] = INBOX_STATUS_RECEIVED;

        rollingHash = EfficientLeftRightKeccak._efficientKeccak(rollingHash, messageHash);

        ++currentL1MessageNumber;
      }
    }

    if (currentL1MessageNumber != _finalMessageNumber) {
      revert L1MessageNumberSynchronizationWrong(_finalMessageNumber, currentL1MessageNumber);
    }

    if (_finalRollingHash != rollingHash) {
      revert L1RollingHashSynchronizationWrong(_finalRollingHash, rollingHash);
    }

    if (currentL1MessageNumber != lastAnchoredL1MessageNumber) {
      lastAnchoredL1MessageNumber = currentL1MessageNumber;
      l1RollingHashes[currentL1MessageNumber] = rollingHash;

      emit L1L2MessageHashesAddedToInbox(_messageHashes);
      emit RollingHashUpdated(currentL1MessageNumber, rollingHash);
    }
  }
}
