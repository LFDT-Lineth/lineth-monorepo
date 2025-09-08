// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.30;

import {IMessageService} from "src/interfaces/IMessageService.sol";

contract TestL2MessageService is IMessageService {
  address internal constant DEFAULT_SENDER_ADDRESS = address(123_456_789);

  /// @dev The temporary message sender set when claiming a message.
  address internal _messageSender;

  /**
   * @dev Thrown when the destination address reverts.
   */
  error MessageSendingFailed(address destination);

  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable {}

  /// @dev Function to simulate the call to the `syncTotalSupplyFromL1` function.
  function syncL1TotalSupply(address _from, address _to, uint256 _value, bytes calldata _calldata) external {
    _messageSender = _from;

    (bool callSuccess, bytes memory returnData) = _to.call{value: _value}(_calldata);
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(0x20, returnData), data_size)
        }
      } else {
        revert MessageSendingFailed(_to);
      }
    }

    _messageSender = DEFAULT_SENDER_ADDRESS;
  }

  /**
   * @dev The _messageSender address is set temporarily when claiming.
   * @return originalSender The original sender stored temporarily at the _messageSender address in storage.
   */
  function sender() external view virtual returns (address originalSender) {
    originalSender = _messageSender;
  }
}
