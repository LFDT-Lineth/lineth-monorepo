// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

contract TestEIP7702Delegation {
  event Log(string message);

  function initialize() external {
    emit Log("Hello, world computer!");
  }
}
