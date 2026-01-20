// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { ClaimMessageV1 } from "../../../messaging/l1/v1/ClaimMessageV1.sol";
import { L1MessageService } from "../../../messaging/l1/L1MessageService.sol";
import { TestSetPauseTypeRoles } from "../security/TestSetPauseTypeRoles.sol";

contract TestL1MessageService is L1MessageService, ClaimMessageV1, TestSetPauseTypeRoles {
  /**
   * @dev Thrown when the message has already been received.
   */
  error MessageAlreadyReceived(bytes32 messageHash);

  address public originalSender;
  bool private reentryDone;

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

  function tryInitialize(
    uint256 _rateLimitPeriod,
    uint256 _rateLimitAmount,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) external {
    __MessageService_init(_rateLimitPeriod, _rateLimitAmount);
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);
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

  function doReentry() external payable {
    address originalAddress;

    (bool success, bytes memory data) = msg.sender.call(abi.encodeWithSignature("sender()"));
    if (success) {
      originalAddress = abi.decode(data, (address));
    }

    if (!reentryDone) {
      (bool succeeded, bytes memory dataInner) = msg.sender.call(
        abi.encodeWithSignature(
          "claimMessage(address,address,uint256,uint256,address,bytes,uint256)",
          originalAddress,
          originalAddress,
          0.05 ether,
          1 ether,
          address(0),
          abi.encodeWithSignature("doReentry()", 1)
        )
      );

      if (succeeded) {
        reentryDone = true;
      } else {
        if (dataInner.length > 0) {
          assembly {
            let data_size := mload(dataInner)
            revert(add(32, dataInner), data_size)
          }
        } else {
          revert("Function call reverted");
        }
      }
    }
  }

  function addFunds() external payable {}
}
