// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

import { BridgedToken } from "./BridgedToken.sol";

/**
 * @title Custom BridgedToken Contract
 * @notice Custom ERC-20 token manually deployed for the Linea TokenBridge.
 */
contract CustomBridgedToken is BridgedToken {
  function initializeV2(
    string memory _tokenName,
    string memory _tokenSymbol,
    uint8 _tokenDecimals,
    address _bridge
  ) public reinitializer(2) {
    __ERC20_init(_tokenName, _tokenSymbol);
    __ERC20Permit_init(_tokenName);
    bridge = _bridge;
    _decimals = _tokenDecimals;
  }
}
