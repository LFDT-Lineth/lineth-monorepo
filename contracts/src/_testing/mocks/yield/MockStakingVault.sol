// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IStakingVault } from "../../../yield/interfaces/vendor/lido/IStakingVault.sol";

contract MockStakingVault is IStakingVault {
  function acceptOwnership() external override {}

  function ossify() external override {}

  function fund() external payable override {}

  error MockWithdrawFailed();

  function withdraw(address _recipient, uint256 _amount) external override {
    (bool success, bytes memory returnData) = _recipient.call{ value: _amount }("");
    if (!success) {
      if (returnData.length > 0) {
        /// @solidity memory-safe-assembly
        assembly {
          revert(add(32, returnData), mload(returnData))
        }
      }
      revert MockWithdrawFailed();
    }
  }

  receive() external payable {}

  function availableBalance() external view returns (uint256) {
    return address(this).balance;
  }

  function pauseBeaconChainDeposits() external override {}

  function resumeBeaconChainDeposits() external override {}

  function triggerValidatorWithdrawals(bytes calldata, uint64[] calldata, address) external payable override {}

  function setDepositor(address _depositor) external {}

  function stagedBalance() external view returns (uint256) {}

  function unstage(uint256 _ether) external {}

  uint256 public transferOwnershipCallCount;

  function transferOwnership(address _newOwner) external {
    transferOwnershipCallCount += 1;
  }
}
