// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { L2MessageService } from "../../../messaging/l2/L2MessageService.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract TestL2MessageService is L2MessageService {
  address public originalSender;
  bool private reentryDone;

  function claimMessageWithoutChecks(
    address _from,
    address _to,
    uint256 _value,
    bytes calldata _calldata
  ) external payable {
    TRANSIENT_MESSAGE_SENDER = _from;
    (bool callSuccess, bytes memory returnData) = _to.call{ value: _value }(_calldata);
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(32, returnData), data_size)
        }
      } else {
        revert MessageSendingFailed(_to);
      }
    }
    TRANSIENT_MESSAGE_SENDER = DEFAULT_MESSAGE_SENDER_TRANSIENT_VALUE;
  }

  function setLastAnchoredL1MessageNumber(uint256 _messageNumber) external {
    lastAnchoredL1MessageNumber = _messageNumber;
  }

  function canSendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable {
    this.sendMessage{ value: msg.value }(_to, _fee, _calldata);
  }

  function setSender() external payable {
    (bool success, bytes memory data) = msg.sender.call(abi.encodeWithSignature("sender()"));
    if (success) {
      originalSender = abi.decode(data, (address));
    }
  }

  function callMessageServiceBase(address _messageServiceBase) external payable {
    (bool success, ) = _messageServiceBase.call(abi.encodeWithSignature("withOnlyMessagingService()"));
    if (!success) {
      revert("Not authorized");
    }
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
          abi.encodeWithSignature("doReentry()")
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

  function makeItRevert() external payable {
    revert();
  }

  function setSlotValue(uint256 _slot, uint256 _value) external {
    assembly {
      sstore(_slot, _value)
    }
  }

  function getSlotValue(uint256 _slot) external view returns (uint256 slotValue) {
    assembly {
      slotValue := sload(_slot)
    }
  }
}
