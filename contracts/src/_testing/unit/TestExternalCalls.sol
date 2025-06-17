// SPDX-License-Identifier: AGPL-3.0

pragma solidity ^0.8.30;

interface ITestExternalCalls {
  function revertWithError() external pure;
  function setValue(uint256 _value) external;
}

contract TestExternalCalls is ITestExternalCalls {
  uint256 public testValue;

  error TestError();

  function revertWithError() external pure {
    revert TestError();
  }

  function setValue(uint256 _value) external {
    testValue = _value;
  }

  fallback() external payable {
    // forced empty revert for code coverage
    revert();
  }

  receive() external payable {}
}
