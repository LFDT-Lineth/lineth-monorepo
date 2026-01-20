// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IYieldManager } from "../../../yield/interfaces/IYieldManager.sol";

contract MockLineaRollup {
  bool private withdrawAllowed;

  function reportNativeYield(uint256 _amount, address _l2YieldRecipient) external {}

  function setWithdrawLSTAllowed(bool _allowed) external {
    withdrawAllowed = _allowed;
  }

  function isWithdrawLSTAllowed() external view returns (bool) {
    return withdrawAllowed;
  }

  function fund() external payable {}

  function callReceiveFundsFromReserve(address _yieldManager, uint256 _amount) external {
    IYieldManager(_yieldManager).receiveFundsFromReserve{ value: _amount }();
  }
}
