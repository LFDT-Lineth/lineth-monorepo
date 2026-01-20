// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IDashboard } from "../../../yield/interfaces/vendor/lido/IDashboard.sol";
import { ICommonVaultOperations } from "../../../yield/interfaces/vendor/lido/ICommonVaultOperations.sol";
import { IStakingVault } from "../../../yield/interfaces/vendor/lido/IStakingVault.sol";

contract MockDashboard is IDashboard {
  IStakingVault private stakingVaultReturn;
  uint256 private totalValueReturn;
  uint256 private liabilitySharesReturn;
  uint256 private withdrawableValueReturn;
  uint256 private accruedFeeReturn;
  uint256 private obligationsFeesToSettleReturn;
  bool isRebalanceVaultWithSharesWithdrawingFromVault;
  bool isDisburseFeeWithdrawingFromVault;

  function setObligationsFeesToSettleReturn(uint256 _value) external {
    obligationsFeesToSettleReturn = _value;
  }

  function setRebalanceVaultWithSharesWithdrawingFromVault(bool _value) external {
    isRebalanceVaultWithSharesWithdrawingFromVault = _value;
  }

  function setIsDisburseFeeWithdrawingFromVault(bool _value) external {
    isDisburseFeeWithdrawingFromVault = _value;
  }

  function setStakingVaultReturn(IStakingVault _value) external {
    stakingVaultReturn = _value;
  }

  function setTotalValueReturn(uint256 _value) external {
    totalValueReturn = _value;
  }

  function setLiabilitySharesReturn(uint256 _value) external {
    liabilitySharesReturn = _value;
  }

  function setWithdrawableValueReturn(uint256 _value) external {
    withdrawableValueReturn = _value;
  }

  function setAccruedFeeReturn(uint256 _value) external {
    accruedFeeReturn = _value;
  }

  function stakingVault() external view override returns (IStakingVault) {
    return stakingVaultReturn;
  }

  function totalValue() external view override returns (uint256) {
    return totalValueReturn;
  }

  function liabilityShares() external view override returns (uint256) {
    return liabilitySharesReturn;
  }

  function withdrawableValue() external view override returns (uint256) {
    return withdrawableValueReturn;
  }

  bool public isVoluntaryDisconnectRevert;

  function setIsVoluntaryDisconnectRevert(bool _val) public {
    isVoluntaryDisconnectRevert = _val;
  }

  function voluntaryDisconnect() external override {
    if (isVoluntaryDisconnectRevert) {
      revert("revert for test");
    }
  }

  function abandonDashboard(address) external override {}

  function mintStETH(address, uint256) external payable override {}

  function rebalanceVaultWithShares(uint256 _amount) external override {
    if (isRebalanceVaultWithSharesWithdrawingFromVault) {
      ICommonVaultOperations vault = ICommonVaultOperations(stakingVaultReturn);
      vault.withdraw(address(0), _amount);
    }
  }

  function rebalanceVaultWithEther(uint256) external payable override {}

  function accruedFee() external view override returns (uint256) {
    return accruedFeeReturn;
  }

  bool public isDisburseFeeRevert;

  function setIsDisburseFeeRevert(bool _val) public {
    isDisburseFeeRevert = _val;
  }

  function disburseFee() external override {
    if (isDisburseFeeRevert) {
      revert("revert for test");
    }
    if (isDisburseFeeWithdrawingFromVault) {
      ICommonVaultOperations vault = ICommonVaultOperations(stakingVaultReturn);
      vault.withdraw(address(0), accruedFeeReturn);
    }
  }

  function reconnectToVaultHub() external override {}

  function fund() external payable override {
    ICommonVaultOperations vault = ICommonVaultOperations(stakingVaultReturn);
    vault.fund{ value: msg.value }();
  }

  error MockWithdrawFailed();

  function withdraw(address _recipient, uint256 _amount) external override {
    ICommonVaultOperations vault = ICommonVaultOperations(stakingVaultReturn);
    vault.withdraw(_recipient, _amount);
  }

  receive() external payable {}

  function pauseBeaconChainDeposits() external override {}

  function resumeBeaconChainDeposits() external override {}

  function triggerValidatorWithdrawals(bytes calldata, uint64[] calldata, address) external payable override {}

  uint256 public transferVaultOwnershipCallCount;

  function transferVaultOwnership(address _newOwner) external {
    transferVaultOwnershipCallCount += 1;
  }

  function obligations() external view returns (uint256 sharesToBurn, uint256 feesToSettle) {
    return (0, obligationsFeesToSettleReturn);
  }
}
