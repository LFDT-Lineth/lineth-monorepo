// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

/**
 * @title Token Minter Interface.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ITokenMinter {
  /**
   * @notice Mints a single token amount for a single recipient.
   * @param _to The address receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function mint(address _to, uint256 _amount) external;

  /**
   * @notice Mints a single token amount for a multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function batchMint(address[] calldata _to, uint256 _amount) external;

  /**
   * @notice Mints a 1:1 amounts for multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amounts The amounts of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function batchMintMultiple(address[] calldata _to, uint256[] calldata _amounts) external;
}
