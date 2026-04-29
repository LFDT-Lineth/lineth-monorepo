// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L1MessageServiceBase } from "./L1MessageServiceBase.sol";
import { L1MessageManager } from "./L1MessageManager.sol";
import { IL1MessageService } from "./interfaces/IL1MessageService.sol";
import { IGenericErrors } from "../../interfaces/IGenericErrors.sol";
import { SparseMerkleTreeVerifier } from "../libraries/SparseMerkleTreeVerifier.sol";
import { MessageHashing } from "../libraries/MessageHashing.sol";

/**
 * @title Contract to manage cross-chain messaging on L1.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract L1MessageService is
  AccessControlUpgradeable,
  L1MessageServiceBase,
  L1MessageManager,
  IL1MessageService,
  IGenericErrors
{
  /// @dev This is currently not in use, but is reserved for future upgrades.
  uint256 public systemMigrationBlock;

  /// @dev Total contract storage is 51 slots including the gap below.
  /// @dev Keep 50 free storage slots for future implementation updates to avoid storage collision.
  uint256[50] private __gap_L1MessageService;

  /**
   * @notice Initialises underlying message service dependencies.
   * @param _rateLimitPeriod The period to rate limit against.
   * @param _rateLimitAmount The limit allowed for withdrawing the period.
   */
  function __MessageService_init(uint256 _rateLimitPeriod, uint256 _rateLimitAmount) internal onlyInitializing {
    __ERC165_init();
    __Context_init();
    __AccessControl_init();
    __RateLimiter_init(_rateLimitPeriod, _rateLimitAmount);

    nextMessageNumber = 1;
  }

  /**
   * @notice Adds a message for sending cross-chain and emits MessageSent.
   * @dev The message number is preset (nextMessageNumber) and only incremented at the end if successful for the next caller.
   * @dev This function should be called with a msg.value = _value + _fee. The fee will be paid on the destination chain.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _calldata The calldata to pass to the recipient.
   */
  function sendMessage(
    address _to,
    uint256 _fee,
    bytes calldata _calldata
  ) external payable virtual whenTypeAndGeneralNotPaused(PauseType.L1_L2) {
    _sendMessage(_to, _fee, _calldata);
  }

  /**
   * @notice Adds a message for sending cross-chain and emits MessageSent.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _calldata The calldata to pass to the recipient.
   */
  function _sendMessage(address _to, uint256 _fee, bytes calldata _calldata) internal virtual {
    if (_to == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_fee > msg.value) {
      revert ValueSentTooLow();
    }

    uint256 messageNumber = nextMessageNumber++;
    uint256 valueSent = msg.value - _fee;

    bytes32 messageHash = MessageHashing._hashMessage(msg.sender, _to, _fee, valueSent, messageNumber, _calldata);

    _addRollingHash(messageNumber, messageHash);

    emit MessageSent(msg.sender, _to, _fee, valueSent, messageNumber, _calldata, messageHash);
  }

  /**
   * @notice Claims and delivers a cross-chain message using a Merkle proof.
   * @dev if tree depth is empty, it will revert with L2MerkleRootDoesNotExist.
   * @dev if tree depth is different than proof size, it will revert with ProofLengthDifferentThanMerkleDepth.
   * @param _params Collection of claim data with proof and supporting data.
   */
  function claimMessageWithProof(
    ClaimMessageWithProofParams calldata _params
  ) public virtual nonReentrant distributeFees(_params.fee, _params.to, _params.data, _params.feeRecipient) {
    _claimMessageWithProof(_params);
  }

  /**
   * @notice Claims and delivers a cross-chain message using a Merkle proof.
   * @param _params Collection of claim data with proof and supporting data.
   */
  function _claimMessageWithProof(ClaimMessageWithProofParams calldata _params) internal virtual {
    _requireTypeAndGeneralNotPaused(PauseType.L2_L1);

    uint256 merkleDepth = l2MerkleRootsDepths[_params.merkleRoot];

    if (merkleDepth == 0) {
      revert L2MerkleRootDoesNotExist();
    }

    if (merkleDepth != _params.proof.length) {
      revert ProofLengthDifferentThanMerkleDepth(merkleDepth, _params.proof.length);
    }

    _setL2L1MessageToClaimed(_params.messageNumber);

    _addUsedAmount(_params.fee + _params.value);

    bytes32 messageLeafHash = MessageHashing._hashMessage(
      _params.from,
      _params.to,
      _params.fee,
      _params.value,
      _params.messageNumber,
      _params.data
    );
    if (
      !SparseMerkleTreeVerifier._verifyMerkleProof(
        messageLeafHash,
        _params.proof,
        _params.leafIndex,
        _params.merkleRoot
      )
    ) {
      revert InvalidMerkleProof();
    }

    TRANSIENT_MESSAGE_SENDER = _params.from;

    (bool callSuccess, bytes memory returnData) = _params.to.call{ value: _params.value }(_params.data);
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(0x20, returnData), data_size)
        }
      } else {
        revert MessageSendingFailed(_params.to);
      }
    }

    TRANSIENT_MESSAGE_SENDER = DEFAULT_MESSAGE_SENDER_TRANSIENT_VALUE;

    emit MessageClaimed(messageLeafHash);
  }

  /**
   * @notice Validates and consumes a message proof.
   * @param _params Collection of claim data with proof and supporting data.
   * @return messageLeafHash The leaf hash of the message.
   */
  function _validateAndConsumeMessageProof(
    ClaimMessageWithProofParams calldata _params
  ) internal virtual returns (bytes32 messageLeafHash) {
    _requireTypeAndGeneralNotPaused(PauseType.L2_L1);

    uint256 merkleDepth = l2MerkleRootsDepths[_params.merkleRoot];

    if (merkleDepth == 0) {
      revert L2MerkleRootDoesNotExist();
    }

    if (merkleDepth != _params.proof.length) {
      revert ProofLengthDifferentThanMerkleDepth(merkleDepth, _params.proof.length);
    }

    _setL2L1MessageToClaimed(_params.messageNumber);

    _addUsedAmount(_params.fee + _params.value);

    messageLeafHash = MessageHashing._hashMessage(
      _params.from,
      _params.to,
      _params.fee,
      _params.value,
      _params.messageNumber,
      _params.data
    );
    if (
      !SparseMerkleTreeVerifier._verifyMerkleProof(
        messageLeafHash,
        _params.proof,
        _params.leafIndex,
        _params.merkleRoot
      )
    ) {
      revert InvalidMerkleProof();
    }
  }

  /**
   * @notice Claims and delivers a cross-chain message.
   * @dev The message sender address is set temporarily in the transient storage when claiming.
   * @return originalSender The message sender address that is stored temporarily in the transient storage when claiming.
   */
  function sender() public view virtual returns (address originalSender) {
    originalSender = TRANSIENT_MESSAGE_SENDER;
  }
}
