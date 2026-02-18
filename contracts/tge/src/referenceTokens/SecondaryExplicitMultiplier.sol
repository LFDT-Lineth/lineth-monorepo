// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.26;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title Soulbound ERC-20 contract for Secondary Explicit Multiplier tokens.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract SecondaryExplicitMultiplier is ERC20, AccessControl {
  error TokenIsSoulBound();

  bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");

  constructor(address minter) ERC20("Secondary Explicit Multiplier", "SEM") {
    _grantRole(DEFAULT_ADMIN_ROLE, minter);
    _grantRole(MINTER_ROLE, minter);
  }

  /**
   * @notice Mints a single token amount for a single recipient.
   * @param _to The address receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function mint(address _to, uint256 _amount) external onlyRole(MINTER_ROLE) {
    _mint(_to, _amount);
  }

  /**
   * @notice Mints a single token amount for a multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function batchMint(address[] calldata _to, uint256 _amount) external onlyRole(MINTER_ROLE) {
    uint256 addressLength = _to.length;

    for (uint256 i; i < addressLength; ) {
      unchecked {
        _mint(_to[i], _amount);
        ++i;
      }
    }
  }

  /**
   * @notice Mints a 1:1 amounts for multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amounts The amounts of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function batchMintMultiple(address[] calldata _to, uint256[] calldata _amounts) external onlyRole(MINTER_ROLE) {
    require(_to.length == _amounts.length, "Array lengths do not match");

    uint256 addressLength = _to.length;
    for (uint256 i; i < addressLength; ) {
      unchecked {
        _mint(_to[i], _amounts[i]);
        ++i;
      }
    }
  }

  /**
   * @notice Overrides and reverts base functions for transfer.
   */
  function transfer(address, uint256) public virtual override returns (bool) {
    revert TokenIsSoulBound();
  }

  /**
   * @notice Overrides and reverts base functions for transferFrom.
   */
  function transferFrom(address, address, uint256) public virtual override returns (bool) {
    revert TokenIsSoulBound();
  }

  /**
   * @notice Overrides and reverts base functions for approve.
   */
  function approve(address, uint256) public virtual override returns (bool) {
    revert TokenIsSoulBound();
  }
}
