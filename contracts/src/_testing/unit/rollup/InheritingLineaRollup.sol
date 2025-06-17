// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.30;
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { LineaRollupBase } from "../../../rollup/LineaRollupBase.sol";
import { L1MessageService } from "../../../messaging/l1/L1MessageService.sol";
import { IMessageService } from "../../../messaging/interfaces/IMessageService.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract InheritingRollup is LineaRollupBase {
  error DirectETHSendingDisallowed();
  error FeeSendingDisallowed();
  error OnlyAllowedSendersToRemoteReceiver();
  error OnlyFromRemoteReceiverToAllowedSender();

  error MessageSenderStateAlreadySet(address sender, bool state);
  error MessageReceiverAlreadySet(address receiver);

  event AllowedMessageSenderStateSet(address sender, bool state);
  event RemoteReceiverSet(address receiver);

  /// @notice The role required to set/remove allowed message senders.
  bytes32 public constant ALLOWED_MESSAGESENDER_SETTING_ROLE = keccak256("ALLOWED_MESSAGESENDER_SETTING_ROLE"); // should be security council restricted

  /// @notice The role required to set the remote receiver.
  bytes32 public constant REMOTE_RECEIVER_SETTER_ROLE = keccak256("REMOTE_RECEIVER_SETTER_ROLE"); // should be security council restricted

  mapping(address remoteReceiver => bool isRemoteReceiver) public remoteReceivers;
  mapping(address sender => bool isAllowedToSendToRemoteReceiver) public allowedMessageSenders;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  function initialize(InitializationData calldata _initializationData) external initializer {
    __LineaRollup_init(_initializationData);
  }

  function setAllowedMessageSenderState(
    address _allowedSender,
    bool _isAllowedToSend
  ) external onlyRole(ALLOWED_MESSAGESENDER_SETTING_ROLE) {
    if (allowedMessageSenders[_allowedSender] == _isAllowedToSend) {
      revert MessageSenderStateAlreadySet(_allowedSender, _isAllowedToSend); // this avoids confusing events
    }

    emit AllowedMessageSenderStateSet(_allowedSender, _isAllowedToSend);
  }

  function setRemoteReceiver(address _receiver) external onlyRole(REMOTE_RECEIVER_SETTER_ROLE) {
    if (remoteReceivers[_receiver]) {
      revert MessageReceiverAlreadySet(_receiver);
    }

    remoteReceivers[_receiver] = true;
    emit RemoteReceiverSet(_receiver);
  }

  function sendMessage(
    address _to,
    uint256 _fee,
    bytes calldata _calldata
  ) external payable override(L1MessageService, IMessageService) whenTypeAndGeneralNotPaused(PauseType.L1_L2) {
    if (msg.value > 0) {
      revert DirectETHSendingDisallowed();
    }

    if (_fee > 0) {
      revert FeeSendingDisallowed();
    }

    if (remoteReceivers[_to] && !allowedMessageSenders[msg.sender]) {
      revert OnlyAllowedSendersToRemoteReceiver();
    }

    _sendMessage(_to, _fee, _calldata);
  }

  /**
   * @notice Claims and delivers a cross-chain message using a Merkle proof.
   * @dev if tree depth is empty, it will revert with L2MerkleRootDoesNotExist.
   * @dev if tree depth is different than proof size, it will revert with ProofLengthDifferentThanMerkleDepth.
   * @param _params Collection of claim data with proof and supporting data.
   */
  function _claimMessageWithProof(ClaimMessageWithProofParams calldata _params) internal virtual override {
    // custom code here

    if (_params.value > 0) {
      revert DirectETHSendingDisallowed();
    }

    if (_params.fee > 0) {
      revert DirectETHSendingDisallowed();
    }

    // is this from the remote bridge? and are they sending back to someone they are allowed to send back to?
    // NB: TBD - This could also be done as pairs vs. open ended.
    if (remoteReceivers[_params.from] && !allowedMessageSenders[_params.to]) {
      revert OnlyFromRemoteReceiverToAllowedSender();
    }

    super._claimMessageWithProof(_params);
  }
}
