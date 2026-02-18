// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.30;

import { IMessageService } from "src/interfaces/IMessageService.sol";

contract TestMessageService is IMessageService {
  /**
   * This contract implements the IMessageService interface and provides the functionality to send messages across
   * chains.
   */
  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable {}

  /// @dev This function is a placeholder to match the IMessageService interface.
  function sender() external pure returns (address) {
    return address(1);
  }
}
