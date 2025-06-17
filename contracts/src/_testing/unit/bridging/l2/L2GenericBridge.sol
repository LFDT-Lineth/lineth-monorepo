// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { MessageServiceBase } from "../../../../messaging/MessageServiceBase.sol";
import { L1GenericBridge } from "../l1/L1GenericBridge.sol"; // Ideally a simple interface.

contract L2GenericBridge is MessageServiceBase {
  // initialize etc

  // add security
  function bridgeEth(address _to, L1GenericBridge.WithdrawalOption _option) external payable {
    // get next message number / nonce;
    uint256 nextMessageNumber;

    messageService.sendMessage(
      remoteSender,
      0,
      abi.encodeCall(L1GenericBridge.receiveDepositedEth, (_to, msg.sender, msg.value, nextMessageNumber, _option))
    );
  }

  // This would be called by the message service (onlyMessagingService)
  // with a check to make sure it came from the L1 Bridge (onlyAuthorizedRemoteSender)
  // The message service checks reentry already
  function receiveDepositedEth(
    address _to,
    address _from,
    uint256 _value,
    uint256 _nonce
  ) external onlyMessagingService onlyAuthorizedRemoteSender {
    // Transfer to the Recipient or customize the process.
  }
}
