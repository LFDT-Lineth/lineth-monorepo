// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.28;

import { MessageServiceBase } from "../../../messaging/MessageServiceBase.sol";

contract TestMessageServiceBase is MessageServiceBase {
  function initialize(address _messageService, address _remoteSender) external initializer {
    __MessageServiceBase_init(_messageService);
    _setRemoteSender(_remoteSender);
  }

  function withOnlyMessagingService() external onlyMessagingService {}

  function withOnlyAuthorizedRemoteSender() external onlyAuthorizedRemoteSender {}

  function tryInitialize(address _messageService, address _remoteSender) external {
    __MessageServiceBase_init(_messageService);
    _setRemoteSender(_remoteSender);
  }

  function testSetRemoteSender(address _remoteSender) external {
    _setRemoteSender(_remoteSender);
  }
}
