// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;
import { CalldataBlobAcceptor } from "../dataAvailability/CalldataBlobAcceptor.sol";
import { L1MessageService } from "../../messaging/l1/L1MessageService.sol";
import { IMessageService } from "../../messaging/interfaces/IMessageService.sol";
import { LineaRollupBase } from "../LineaRollupBase.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract CalldataOnlyDaRollup is LineaRollupBase, CalldataBlobAcceptor {
  /**
   * @dev These are examples of custom events and functionality.
   */

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

  function initialize(BaseInitializationData calldata _initializationData) external initializer {
    bytes32 genesisShnarf = _computeShnarf(
      EMPTY_HASH,
      EMPTY_HASH,
      _initializationData.initialStateRootHash,
      EMPTY_HASH,
      EMPTY_HASH
    );

    _blobShnarfExists[genesisShnarf] = SHNARF_EXISTS_DEFAULT_VALUE;
    __LineaRollup_init(_initializationData, genesisShnarf);
  }

  /**
   * @dev This is an example of new custom functionality.
   */
  function setAllowedMessageSenderState(
    address _allowedSender,
    bool _isAllowedToSend
  ) external onlyRole(ALLOWED_MESSAGESENDER_SETTING_ROLE) {
    if (allowedMessageSenders[_allowedSender] == _isAllowedToSend) {
      revert MessageSenderStateAlreadySet(_allowedSender, _isAllowedToSend); // this avoids confusing events
    }

    emit AllowedMessageSenderStateSet(_allowedSender, _isAllowedToSend);
  }

  /**
   * @dev This is an example of new custom functionality.
   */
  function setRemoteReceiver(address _receiver) external onlyRole(REMOTE_RECEIVER_SETTER_ROLE) {
    if (remoteReceivers[_receiver]) {
      revert MessageReceiverAlreadySet(_receiver);
    }

    remoteReceivers[_receiver] = true;
    emit RemoteReceiverSet(_receiver);
  }

  /**
   * @dev This is an example override of the sendMessage to add custom functionality.
   */
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
   * @dev This is an example override of the _claimMessageWithProof to add custom functionality.
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
