// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

contract ErrorAndDestructionTesting {
  function externalRevert() external pure {
    revert("OPCODE FD");
  }

  function callmeToSelfDestruct() external {
    selfdestruct(payable(address(0)));
  }
}
