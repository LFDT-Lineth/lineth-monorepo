// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.30;

interface IL2LineaToken {
  /// @dev Thrown when the caller address is not the token bridge address.
  error CallerIsNotTokenBridge();

  /// @dev Thrown when a more recent sync time from L1 has been synchronized.
  error LastSyncMoreRecent();

  /// @dev Thrown when a string value is empty.
  error EmptyStringNotAllowed();

  /**
   * @notice Emitted when the L1 token address is set.
   * @param l1TokenAddress The address of the L1 token contract.
   */
  event L1TokenAddressSet(address l1TokenAddress);
  /**
   * @notice Emitted when the L2 message service address is set.
   * @param l2MessageService The L2 message service address.
   */
  event L2MessageServiceSet(address l2MessageService);

  /**
   * @notice Emitted when the L1 Linea Canonical Token Bridge is set.
   * @param lineaCanonicalTokenBridge The Linea Canonical Token Bridge address.
   */
  event LineaCanonicalTokenBridgeSet(address lineaCanonicalTokenBridge);

  /**
   * @notice Emitted when the token metadata is set.
   * @param name The name of the token.
   * @param symbol The symbol of the token.
   */
  event TokenMetadataSet(string name, string symbol);

  /**
   * @notice Emitted when the L1 Linea token total supply is synchronized on L2.
   * @param l1LineaTokenTotalSupplySyncTime The L1 block timestamp synchronized.
   * @param l1LineaTokenSupply The L1 Linea Token total supply synchronized.
   */
  event L1LineaTokenTotalSupplySynced(uint256 l1LineaTokenTotalSupplySyncTime, uint256 l1LineaTokenSupply);

  /**
   * @notice Mints the Linea token.
   * @dev NB: Only the L2 token bridge can call this function.
   * @param _account Account being minted for.
   * @param _amount The amount being minted for the account.
   */
  function mint(address _account, uint256 _amount) external;

  /**
   * @notice Burns the Linea token.
   * @dev NB: Only the L2 token bridge can call this function.
   * @dev Approval for the burn amount must be provided before this is invoked.
   * @param _account Account being burned for.
   * @param _value The amount being burned for the account.
   */
  function burn(address _account, uint256 _value) external;

  /**
   * @notice Synchronizes the total supply of the L1 Linea token from L1 Ethereum.
   * @dev NB: This function can only be called by the Linea Message Service.
   * @dev NB: This function must have originated from the Linea token on L1 Ethereum.
   * @param _l1LineaTokenTotalSupplySyncTime The L1 block.timestamp when the Linea token on L1 total supply was computed.
   * @param _l1LineaTokenSupply The total supply of the L1 Linea token.
   */
  function syncTotalSupplyFromL1(uint256 _l1LineaTokenTotalSupplySyncTime, uint256 _l1LineaTokenSupply) external;
}
