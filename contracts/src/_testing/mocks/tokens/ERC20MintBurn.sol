// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract MockERC20MintBurn is ERC20 {
  constructor(string memory _tokenName, string memory _tokenSymbol) ERC20(_tokenName, _tokenSymbol) {}

  function mint(address _account, uint256 _amount) public returns (bool) {
    _mint(_account, _amount);
    return true;
  }

  function burn(address _account, uint256 _amount) public returns (bool) {
    _burn(_account, _amount);
    return true;
  }
}
