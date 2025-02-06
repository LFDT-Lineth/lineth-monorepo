// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.19;

import { ReentrancyContract } from "./ReentrancyContract.sol";

contract MaliciousERC777 {
  mapping(address => uint256) public balanceOf;
  ReentrancyContract private reentrancyContract;

  constructor(address _reentrancyContract) {
    reentrancyContract = ReentrancyContract(_reentrancyContract);
  }

  function mint(address _to, uint256 _amount) external {
    balanceOf[_to] += _amount;
  }

  function transferFrom(address _from, address _to, uint256 _amount) external {
    reentrancyContract.beforeTokenTransfer();

    balanceOf[_from] -= _amount;
    balanceOf[_to] += _amount;
  }

  function name() external pure returns (string memory) {
    return "Token";
  }

  function symbol() external pure returns (string memory) {
    return "Token";
  }

  function decimals() external pure returns (uint8) {
    return 18;
  }
}
