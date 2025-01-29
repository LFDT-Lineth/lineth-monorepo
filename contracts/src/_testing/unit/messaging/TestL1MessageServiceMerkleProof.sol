// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.26;

import { L1MessageService } from "../../../messaging/l1/L1MessageService.sol";
import { IL1MessageService } from "../../../messaging/l1/interfaces/IL1MessageService.sol";
import { TestSetPauseTypeRoles } from "../security/TestSetPauseTypeRoles.sol";

interface ITestL1MessageService {
  function claimMessageWithProof(IL1MessageService.ClaimMessageWithProofParams calldata _params) external;
}

contract TestL1MessageServiceMerkleProof is L1MessageService, TestSetPauseTypeRoles {
  address public originalSender;
  bool private reentryDone;

  /**
   * @dev Thrown when the message has already been received.
   */
  error MessageAlreadyReceived(bytes32 messageHash);

  function initialize(
    uint256 _rateLimitPeriod,
    uint256 _rateLimitAmount,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) public initializer {
    _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);
    __MessageService_init(_rateLimitPeriod, _rateLimitAmount);
  }

  function tryInitialize(uint256 _rateLimitPeriod, uint256 _rateLimitAmount) external {
    __MessageService_init(_rateLimitPeriod, _rateLimitAmount);
  }

  // @dev - the this. sendMessage is because the function is an "external" call and not wrapped
  function canSendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable {
    this.sendMessage{ value: msg.value }(_to, _fee, _calldata);
  }

  function addL2L1MessageHash(bytes32 _messageHash) external {
    if (inboxL2L1MessageStatus[_messageHash] != INBOX_STATUS_UNKNOWN) {
      revert MessageAlreadyReceived(_messageHash);
    }

    inboxL2L1MessageStatus[_messageHash] = INBOX_STATUS_RECEIVED;
  }

  function setSender() external payable {
    (bool success, bytes memory data) = msg.sender.call(abi.encodeWithSignature("sender()"));
    if (success) {
      originalSender = abi.decode(data, (address));
    }
  }

  function sendNewMessage() external payable {
    this.sendMessage{ value: 1 wei }(address(this), 1, "0x");
  }

  function doReentryWithParams(IL1MessageService.ClaimMessageWithProofParams calldata _params) external payable {
    ITestL1MessageService messageService = ITestL1MessageService(msg.sender);
    messageService.claimMessageWithProof(_params);
  }

  function doReentry() external payable {
    address originalAddress;

    (bool success, bytes memory data) = msg.sender.call(abi.encodeWithSignature("sender()"));
    if (success) {
      originalAddress = abi.decode(data, (address));
    }

    if (!reentryDone) {
      IL1MessageService(msg.sender);
    }
  }

  function addFunds() external payable {}

  function setL2L1MessageToClaimed(uint256 _index) external {
    _setL2L1MessageToClaimed(_index);
  }

  function addL2MerkleRoots(bytes32[] calldata _newRoot, uint256 _treeDepth) external {
    _addL2MerkleRoots(_newRoot, _treeDepth);
  }
}
