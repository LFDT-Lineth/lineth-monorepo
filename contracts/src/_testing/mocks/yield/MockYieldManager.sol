// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { ILineaRollupYieldExtension } from "../../../yield/interfaces/ILineaRollupYieldExtension.sol";
import { IL1MessageService } from "../../../messaging/l1/interfaces/IL1MessageService.sol";

/// @dev Mock YieldManager contract for unit testing of LineaRollup
contract MockYieldManager {
  bytes private reentryCalldata;
  address private reentryYieldProvider;
  bool private shouldAttemptReentry;

  event LSTWithdrawalFlag(bool indexed flag);

  function receiveFundsFromReserve() external payable {}

  function setReentryData(
    IL1MessageService.ClaimMessageWithProofParams calldata _params,
    address _yieldProvider
  ) external {
    reentryCalldata = abi.encode(_params);
    reentryYieldProvider = _yieldProvider;
    shouldAttemptReentry = true;
  }

  function withdrawLST(address /*_yieldProvider*/, uint256 /*_amount*/, address /*_recipient*/) external {
    bool isWithdrawalToggleOn = ILineaRollupYieldExtension(msg.sender).isWithdrawLSTAllowed();
    emit LSTWithdrawalFlag(isWithdrawalToggleOn);
    if (!shouldAttemptReentry) {
      return;
    }

    shouldAttemptReentry = false;

    IL1MessageService.ClaimMessageWithProofParams memory params = abi.decode(
      reentryCalldata,
      (IL1MessageService.ClaimMessageWithProofParams)
    );

    ILineaRollupYieldExtension(msg.sender).claimMessageWithProofAndWithdrawLST(params, reentryYieldProvider);
  }
}
