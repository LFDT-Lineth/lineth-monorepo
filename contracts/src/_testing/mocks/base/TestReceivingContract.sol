// SPDX-License-Identifier: AGPL-3.0

pragma solidity ^0.8.19;

contract TestReceivingContract {
  fallback() external payable {}

  receive() external payable {}
}
