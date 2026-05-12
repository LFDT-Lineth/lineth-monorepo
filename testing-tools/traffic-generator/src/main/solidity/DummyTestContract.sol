// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract DummyTestContract is ERC20, Ownable {
  constructor(string memory name_, string memory symbol_) ERC20(name_, symbol_)  {}

  function mint(address _to, uint256 _amount) external onlyOwner {
    _mint(_to, _amount);
  }

  function ethTransfer(address _to) external payable {
    _to.call{value:msg.value}("");
  }

  function batchMint(address[] calldata _to, uint256 _amount) external onlyOwner {
    uint256 addressLength = _to.length;

    for (uint256 i; i < addressLength; ) {
      unchecked {
        _mint(_to[i], _amount);
        ++i;
      }
    }
  }

  function batchMintMultiple(address[] calldata _to, uint256[] calldata _amounts) external onlyOwner {
    require(_to.length == _amounts.length, "Array lengths do not match");

    uint256 addressLength = _to.length;
    for (uint256 i; i < addressLength; ) {
      unchecked {
        _mint(_to[i], _amounts[i]);
        ++i;
      }
    }
  }
}
