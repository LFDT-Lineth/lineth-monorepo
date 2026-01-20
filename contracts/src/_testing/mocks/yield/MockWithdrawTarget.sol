// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IYieldManager } from "../../../yield/interfaces/IYieldManager.sol";

interface IMockWithdrawTarget {
  function withdraw(uint256 _amount, address _recipient) external;
}

contract MockWithdrawTarget is IMockWithdrawTarget {
  error MockWithdrawFailed();

  function withdraw(uint256 _amount, address _recipient) public {
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
}
