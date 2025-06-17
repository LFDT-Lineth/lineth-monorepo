// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

/// @dev Test contract to test LXP-L minting
interface ITransferSurgeXP {
  function transfer(address _address, uint256 _amount) external returns (bool);
}

contract TestLineaSurgeXP {
  /// @dev In a real contract, this would be permissioned to avoid abuse.
  function testTransfer(address _contractAddress, address _recipient, uint256 _amount) external returns (bool) {
    return ITransferSurgeXP(_contractAddress).transfer(_recipient, _amount);
  }
}
