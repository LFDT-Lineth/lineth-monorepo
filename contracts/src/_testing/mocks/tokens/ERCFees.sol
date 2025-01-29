// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

uint16 constant FEES_PERCENTAGE_MULTIPLIER = 10000;

contract ERC20Fees is ERC20 {
  uint16 public feePercentage;

  /**
   * @dev Constructor that gives _msgSender() all of existing tokens.
   * @param _tokenName string memory token name
   * @param _tokenSymbol string memory token symbol
   * @param _feePercentage uint16 fee percentage with FEE_PERCENTAGE_MULTIPLIER
   */
  constructor(
    string memory _tokenName,
    string memory _tokenSymbol,
    uint16 _feePercentage
  ) ERC20(_tokenName, _tokenSymbol) {
    feePercentage = _feePercentage;
  }

  function mint(address _account, uint256 _amount) public returns (bool) {
    _mint(_account, _amount);
    return true;
  }

  function _transfer(address _sender, address _recipient, uint256 _amount) internal virtual override {
    _burn(_sender, (_amount * feePercentage) / FEES_PERCENTAGE_MULTIPLIER);
    super._transfer(
      _sender,
      _recipient,
      (_amount * (FEES_PERCENTAGE_MULTIPLIER - feePercentage)) / FEES_PERCENTAGE_MULTIPLIER
    );
  }

  function burn(address _account, uint256 _amount) public returns (bool) {
    _burn(_account, _amount);
    return true;
  }
}
