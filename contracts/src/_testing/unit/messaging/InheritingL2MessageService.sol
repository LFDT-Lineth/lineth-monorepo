// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { L2MessageServiceBase } from "../../../messaging/l2/L2MessageServiceBase.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract InheritingL2MessageService is L2MessageServiceBase {
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

  /**
   * @notice Initializes underlying message service dependencies.
   * @param _rateLimitPeriod The period to rate limit against.
   * @param _rateLimitAmount The limit allowed for withdrawing the period.
   * @param _defaultAdmin The account to be given DEFAULT_ADMIN_ROLE on initialization.
   * @param _roleAddresses The list of addresses to grant roles to.
   * @param _pauseTypeRoles The list of pause type roles.
   * @param _unpauseTypeRoles The list of unpause type roles.
   */
  function initialize(
    uint256 _rateLimitPeriod,
    uint256 _rateLimitAmount,
    address _defaultAdmin,
    RoleAddress[] calldata _roleAddresses,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) external initializer {
    __L2MessageService_init(
      _rateLimitPeriod,
      _rateLimitAmount,
      _defaultAdmin,
      _roleAddresses,
      _pauseTypeRoles,
      _unpauseTypeRoles
    );
  }

  function setRemoteReceiver(address _receiver) external onlyRole(REMOTE_RECEIVER_SETTER_ROLE) {
    if (remoteReceivers[_receiver]) {
      revert MessageReceiverAlreadySet(_receiver);
    }

    remoteReceivers[_receiver] = true;
    emit RemoteReceiverSet(_receiver);
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

  function sendMessage(
    address _to,
    uint256 _fee,
    bytes calldata _calldata
  ) external payable override whenTypeAndGeneralNotPaused(PauseType.L2_L1) {
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

  function _claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    bytes calldata _calldata,
    uint256 _nonce
  ) internal override {
    // This may be an issue with migration - TBD
    if (_value > 0) {
      revert DirectETHSendingDisallowed();
    }

    // TBC: if running an L1 postman...
    // if (_params.fee > 0) {
    //   revert DirectETHSendingDisallowed();
    // }

    // is this from the remote bridge? and are they sending back to someone they are allowed to send back to?
    // NB: TBD - This might need to be done as pairs vs. open ended.
    if (remoteReceivers[_from] && !allowedMessageSenders[_to]) {
      revert OnlyFromRemoteReceiverToAllowedSender();
    }

    super._claimMessage(_from, _to, _fee, _value, _calldata, _nonce);
  }
}
