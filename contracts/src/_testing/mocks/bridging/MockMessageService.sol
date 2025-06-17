// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { IMessageService } from "../../../messaging/interfaces/IMessageService.sol";

contract MockMessageService is IMessageService {
  uint256 public constant CALL_GAS_LIMIT = 1000000;
  address internal messageSender = address(0);

  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable {
    require(msg.value >= _fee, "MessageService: Value too low");
    messageSender = msg.sender;
    uint256 _value = msg.value - _fee;
    (bool success, bytes memory result) = _to.call{ value: _value, gas: CALL_GAS_LIMIT }(_calldata);

    // This is used to return the same revert message as the contract called returns it
    if (success == false) {
      assembly {
        revert(add(result, 32), mload(result))
      }
    }
  }

  // When called within the context of the delivered call returns the sender from the other layer
  // otherwise returns the zero address
  function sender() external view returns (address) {
    return messageSender;
  }

  // Placeholder
  function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
  ) external {}
}
