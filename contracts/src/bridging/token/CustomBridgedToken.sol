// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { BridgedToken } from "./BridgedToken.sol";

/**
 * @title Custom BridgedToken Contract
 * @notice Custom ERC-20 token manually deployed for the Linea TokenBridge.
 */
contract CustomBridgedToken is BridgedToken {
  /**
   * @notice Reinitializes an existing deployed contract.
   * @dev NB: If this is being used for a fresh deploy, DO NOT target the initialize function,
   *          only call the initializeV2 (unless you call both in the same transaction) because:
   *          1. It is cheaper gas wise.
   *          2. It avoids a front-running attack where someone else can call initializeV2 first.
   * @param _tokenName The token name.
   * @param _tokenSymbol The token symbol.
   * @param _tokenDecimals The token decimals.
   * @param _bridge The TokenBridge Address.
   */
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
