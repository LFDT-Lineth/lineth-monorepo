// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IVaultHub } from "../../../yield/interfaces/vendor/lido/IVaultHub.sol";
import { ICommonVaultOperations } from "../../../yield/interfaces/vendor/lido/ICommonVaultOperations.sol";

contract MockVaultHub is IVaultHub {
  bool isVaultConnectedReturn;
  bool isPendingDisconnectReturn;
  bool isSettleLidoFeesWithdrawingFromVault;
  bool isSettleLidoFeesReverting;
  uint256 settleVaultObligationAmount;
  error Revert();

  function setIsVaultConnectedReturn(bool _val) external {
    isVaultConnectedReturn = _val;
  }

  function setIsPendingDisconnectReturn(bool _val) external {
    isPendingDisconnectReturn = _val;
  }

  function setIsSettleLidoFeesWithdrawingFromVault(bool _value) external {
    isSettleLidoFeesWithdrawingFromVault = _value;
  }

  function setIsSettleLidoFeesReverting(bool _value) external {
    isSettleLidoFeesReverting = _value;
  }

  function setSettleVaultObligationAmount(uint256 _val) external {
    settleVaultObligationAmount = _val;
  }

  function settleLidoFees(address _vault) external override {
    if (isSettleLidoFeesReverting) {
      revert Revert();
    }
    if (isSettleLidoFeesWithdrawingFromVault) {
      ICommonVaultOperations vault = ICommonVaultOperations(_vault);
      vault.withdraw(address(0), settleVaultObligationAmount);
    }
  }

  function isVaultConnected(address _vault) external view override returns (bool) {
    return isVaultConnectedReturn;
  }

  function isPendingDisconnect(address _vault) external view override returns (bool) {
    return isPendingDisconnectReturn;
  }
}
