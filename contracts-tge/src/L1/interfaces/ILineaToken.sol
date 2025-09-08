// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.30;

interface ILineaToken {
  /// @dev Thrown when an address is the zero address.
  error ZeroAddressNotAllowed();

  /// @dev Thrown when a string value is empty.
  error EmptyStringNotAllowed();

  /**
   * @notice Emitted when the token metadata is set.
   * @param name The name of the token.
   * @param symbol The symbol of the token.
   */
  event TokenMetadataSet(string name, string symbol);

  /**
   * @notice Emitted when the L2 token address is set.
   * @param l2TokenAddress The address of the L2 token contract.
   */
  event L2TokenAddressSet(address l2TokenAddress);

  /**
   * @notice Emitted when the L1 message service address is set.
   * @param l1MessageService The L1 message service address.
   */
  event L1MessageServiceSet(address l1MessageService);

  /**
   * @notice Emitted when the L1 total supply synchronization starts.
   * @param l1TotalSupply The total supply of the L1 token.
   */
  event L1TotalSupplySyncStarted(uint256 l1TotalSupply);

  /**
   * @notice Mints the Linea token.
   * @dev NB: Only those with MINTER_ROLE can call this function.
   * @param _account Account being minted for.
   * @param _amount The amount being minted for the account.
   */
  function mint(address _account, uint256 _amount) external;

  /**
   * @notice Synchronizes the total supply of the L1 token to the L2 token.
   * @dev This function sends a message to the L2 token contract to sync the total supply.
   * @dev NB: This function is permissionless on purpose, allowing anyone to trigger the sync.
   * @dev This function can only be called after the L2 token address has been set.
   */
  function syncTotalSupplyToL2() external;
}
