// SPDX-License-Identifier: MIT

pragma solidity 0.8.19;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TestERC20
 * @dev Simple ERC-20 Token example.
 */

contract TestERC20 is ERC20, Ownable {
  /**
   * @dev Constructor that gives msg.sender all of existing tokens.
   */

  constructor(string memory _name, string memory _symbol, uint256 _initialSupply) ERC20(_name, _symbol) {
    _mint(msg.sender, _initialSupply);
  }

  /**
   * @dev Function to mint tokens
   * @param _to The address that will receive the minted tokens.
   * @param _amount The amount of tokens to mint.
   */

  function mint(address _to, uint256 _amount) public {
    _mint(_to, _amount);
  }

  /**
   * @dev Function to burn tokens
   * @param _amount The amount of tokens to burn.
   */

  function burn(uint256 _amount) public {
    _burn(msg.sender, _amount);
  }
}
