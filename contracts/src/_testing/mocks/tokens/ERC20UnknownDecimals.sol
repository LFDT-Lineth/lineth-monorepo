// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract ERC20UnknownDecimals is ERC20 {
  constructor(string memory _tokenName, string memory _tokenSymbol) ERC20(_tokenName, _tokenSymbol) {}

  function decimals() public view virtual override returns (uint8) {
    revert("Forced failure");
  }

  function mint(address _account, uint256 _value) public returns (bool) {
    _mint(_account, _value);
    return true;
  }
}
