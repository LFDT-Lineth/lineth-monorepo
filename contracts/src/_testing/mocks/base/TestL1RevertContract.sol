// SPDX-License-Identifier: AGPL-3.0

pragma solidity 0.8.28;

contract TestL1RevertContract {
  function errorWithMessage() external pure {
    revert("Reverting with receive");
  }

  function errorWithoutMessage() external pure {
    revert();
  }
}
