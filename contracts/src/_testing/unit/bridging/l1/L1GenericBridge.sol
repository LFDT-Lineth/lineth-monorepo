// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { MessageServiceBase } from "../../../../messaging/MessageServiceBase.sol";
import { L2GenericBridge } from "../l2/L2GenericBridge.sol"; // Ideally a simple interface.

// Contract could be made upgradable with access control etc by implementors.
contract L1GenericBridge is MessageServiceBase {
  uint256 public withdrawalDelay;
  address public escrowAddress;

  enum BridingStatus {
    UNKNOWN,
    REQUESTED,
    COMPLETE
  }

  enum WithdrawalOption {
    WAIT,
    IMMEDIATE
  }

  mapping(bytes32 bridgingHash => BridingStatus status) public bridgingStatuses;

  // Implementors would add access control.
  function setEscrowAddress(address _escrowAddress) external {
    if (escrowAddress == _escrowAddress) {
      revert("already set"); // error averts weird events for set address.
    }

    escrowAddress = _escrowAddress;
    // emit events
  }

  // Implementors would add access control.
  function setWithdrawalDelay(uint256 _withdrawalDelay) external {
    if (withdrawalDelay == _withdrawalDelay) {
      revert("already set"); // error averts weird events for set address.
    }

    withdrawalDelay = _withdrawalDelay;
    // emit events
  }

  // Implementors would add any additional modifiers ( pause etc ).
  function bridgeEth(address _to) external payable {
    // get next message number / nonce - this should come from the message service.
    uint256 nextMessageNumber;

    messageService.sendMessage(
      remoteSender,
      0,
      abi.encodeCall(L2GenericBridge.receiveDepositedEth, (_to, msg.sender, msg.value, nextMessageNumber))
    );

    // emit events etc
  }

  // This would be called by the message service (onlyMessagingService)
  // with a check to make sure it came from the L1 Bridge (onlyAuthorizedRemoteSender)
  // The message service checks reentry already
  function receiveDepositedEth(
    address _to,
    address _from,
    uint256 _value,
    uint256 _nonce,
    WithdrawalOption _option
  ) external onlyMessagingService onlyAuthorizedRemoteSender {
    uint256 minWithdrawTime = block.timestamp;

    if (_option == WithdrawalOption.WAIT) {
      unchecked {
        minWithdrawTime += withdrawalDelay;
      }
    }

    // hashing to be optimized
    bytes32 bridgingHash = keccak256(abi.encode(_to, _from, _value, _nonce, _option, minWithdrawTime));

    if (bridgingStatuses[bridgingHash] != BridingStatus.UNKNOWN) {
      // reentry not allowed
      revert();
    }

    if (_option == WithdrawalOption.WAIT) {
      bridgingStatuses[bridgingHash] = BridingStatus.REQUESTED;
      // emit requested
    } else {
      bridgingStatuses[bridgingHash] = BridingStatus.COMPLETE;
      // call escrowAddress for immediate transfer.
    }
  }

  function completeWithdrawal(
    address _to,
    address _from,
    uint256 _value,
    uint256 _nonce,
    WithdrawalOption _option,
    uint256 _minWithdrawTime
  ) external payable {
    if (block.timestamp < _minWithdrawTime) {
      revert("too early");
    }

    if (_value != msg.value) {
      revert("value is wrong");
    }

    // hashing to be optimized
    bytes32 bridgingHash = keccak256(abi.encode(_to, _from, _value, _nonce, _option, _minWithdrawTime));

    if (bridgingStatuses[bridgingHash] != BridingStatus.REQUESTED) {
      // reentry not allowed
      revert();
    }

    bridgingStatuses[bridgingHash] = BridingStatus.COMPLETE;

    (bool callSuccess, bytes memory returnData) = _to.call{ value: _value }("");
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(32, returnData), data_size)
        }
      } else {
        //revert MessageSendingFailed(_to);
      }
    }
  }

  // function: add ETH for user withdrawal (admin only)
  function addWithdrawalEth() external payable {}
}
