// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IMessageService {
  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable;
}

contract MultiMessageSender {
  function sendMultipleMessages(address messageService, address to, uint256 fee, uint256 count) external payable {
    for (uint256 i = 0; i < count; i++) {
      IMessageService(messageService).sendMessage{ value: fee }(to, fee, "");
    }
  }
}
