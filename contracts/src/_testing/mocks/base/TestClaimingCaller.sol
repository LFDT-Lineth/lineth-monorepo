// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { IMessageService } from "../../../messaging/interfaces/IMessageService.sol";

contract TestClaimingCaller {
  address private expectedAddress;

  constructor(address _expectedAddress) {
    expectedAddress = _expectedAddress;
  }
  receive() external payable {
    address originalSender = IMessageService(msg.sender).sender();
    assert(originalSender == expectedAddress);
  }
}
