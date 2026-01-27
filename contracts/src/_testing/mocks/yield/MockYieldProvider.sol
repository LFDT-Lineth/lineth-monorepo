// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IYieldProvider } from "../../../yield/interfaces/IYieldProvider.sol";
import {
  ProgressOssificationResult,
  YieldProviderRegistration,
  YieldProviderVendor
} from "../../../yield/interfaces/YieldTypes.sol";
import { IGenericErrors } from "../../../interfaces/IGenericErrors.sol";

import { YieldManagerStorageLayout } from "../../../yield/YieldManagerStorageLayout.sol";
import { MockYieldProviderStorageLayout } from "./MockYieldProviderStorageLayout.sol";
import { IMockWithdrawTarget } from "./MockWithdrawTarget.sol";

contract MockYieldProvider is IYieldProvider, MockYieldProviderStorageLayout {
  function withdrawableValue(address _yieldProvider) external payable returns (uint256 availableBalance) {
    return withdrawableValueReturnVal(_yieldProvider);
  }

  function syncLSTLiabilityPrincipal(address _yieldProvider) external {}

  error FundMockWithdrawTargetFailed();

  // Route to MockWithdrawTarget
  function fundYieldProvider(address _yieldProvider, uint256 _amount) external {
    address mockWithdrawTargetAddress = getMockWithdrawTarget(_yieldProvider);
    (bool success, bytes memory returnData) = mockWithdrawTargetAddress.call{ value: _amount }("");
    if (!success) {
      revert FundMockWithdrawTargetFailed();
    }
  }

  function reportYield(
    address _yieldProvider
  ) external returns (uint256 newReportedYield, uint256 outstandingNegativeYield) {
    return (
      reportYieldReturnVal_NewReportedYield(_yieldProvider),
      reportYieldReturnVal_OutstandingNegativeYield(_yieldProvider)
    );
  }

  function payLSTPrincipal(
    address _yieldProvider,
    uint256 _availableFunds
  ) external returns (uint256 lstPrincipalPaid) {
    return payLSTPrincipalReturnVal(_yieldProvider);
  }

  function unstake(address _yieldProvider, bytes memory _withdrawalParams) external payable {}

  function unstakePermissionless(
    address _yieldProvider,
    uint256 _requiredUnstakeAmount,
    uint64 _validatorIndex,
    uint64 _slot,
    bytes calldata _withdrawalParams,
    bytes calldata _withdrawalParamsProof
  ) external payable returns (uint256 unstakedAmount) {
    return unstakePermissionlessReturnVal(_yieldProvider);
  }

  function withdrawFromYieldProvider(address _yieldProvider, uint256 _amount) external {
    IMockWithdrawTarget mockWithdrawTarget = IMockWithdrawTarget(getMockWithdrawTarget(_yieldProvider));
    mockWithdrawTarget.withdraw(_amount, address(this));
  }

  function beforeWithdrawFromYieldProvider(
    address _yieldProvider,
    bool _isPermissionlessReserveDeficitWithdrawal
  ) external {}

  function pauseStaking(address _yieldProvider) external {}

  function unpauseStaking(address _yieldProvider) external {}

  function withdrawLST(address _yieldProvider, uint256 _amount, address _recipient) external {}

  function initiateOssification(address _yieldProvider) external {
    if (getIsInitiateOssificationReverting(_yieldProvider)) {
      revert("initiateOssification: forced revert for testing");
    }
  }

  function progressPendingOssification(
    address _yieldProvider
  ) external returns (ProgressOssificationResult progressOssificationResult) {
    return getProgressPendingOssificationReturnVal(_yieldProvider);
  }

  function initializeVendorContracts(
    bytes memory _vendorInitializationData
  ) external returns (YieldProviderRegistration memory registrationData) {
    return getInitializeVendorContractsReturnVal();
  }

  function exitVendorContracts(address _yieldProvider, bytes memory _vendorExitData) external {}
}
