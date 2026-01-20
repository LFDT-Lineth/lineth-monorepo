// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { YieldManager } from "../../../yield/YieldManager.sol";
import { MockYieldProviderStorageLayout } from "../../mocks/yield/MockYieldProviderStorageLayout.sol";
import { TestLidoStVaultYieldProvider } from "./TestLidoStVaultYieldProvider.sol";
import { IYieldProvider } from "../../../yield/interfaces/IYieldProvider.sol";
import { YieldProviderVendor, YieldProviderRegistration } from "../../../yield/interfaces/YieldTypes.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract TestYieldManager is YieldManager, MockYieldProviderStorageLayout {
  constructor(address _l1MessageService) YieldManager(_l1MessageService) {}

  function getL1MessageService() external view returns (address) {
    return L1_MESSAGE_SERVICE;
  }

  function getMinimumWithdrawalReservePercentageBps() external view returns (uint16) {
    return _getYieldManagerStorage().minimumWithdrawalReservePercentageBps;
  }

  function setMinimumWithdrawalReservePercentageBps(uint16 _minimumWithdrawalReservePercentageBps) external {
    _getYieldManagerStorage().minimumWithdrawalReservePercentageBps = _minimumWithdrawalReservePercentageBps;
  }

  function getTargetWithdrawalReservePercentageBps() external view returns (uint16) {
    return _getYieldManagerStorage().targetWithdrawalReservePercentageBps;
  }

  function setTargetWithdrawalReservePercentageBps(uint16 _targetWithdrawalReservePercentageBps) external {
    _getYieldManagerStorage().targetWithdrawalReservePercentageBps = _targetWithdrawalReservePercentageBps;
  }

  function getMinimumWithdrawalReserveAmount() external view returns (uint256) {
    return _getYieldManagerStorage().minimumWithdrawalReserveAmount;
  }

  function setMinimumWithdrawalReserveAmount(uint256 _minimumWithdrawalReserveAmount) external {
    _getYieldManagerStorage().minimumWithdrawalReserveAmount = _minimumWithdrawalReserveAmount;
  }

  function getTargetWithdrawalReserveAmount() external view returns (uint256) {
    return _getYieldManagerStorage().targetWithdrawalReserveAmount;
  }

  function setTargetWithdrawalReserveAmount(uint256 _targetWithdrawalReserveAmount) external {
    _getYieldManagerStorage().targetWithdrawalReserveAmount = _targetWithdrawalReserveAmount;
  }

  function getUserFundsInYieldProvidersTotal() external view returns (uint256) {
    return _getYieldManagerStorage().userFundsInYieldProvidersTotal;
  }

  function setUserFundsInYieldProvidersTotal(uint256 _userFundsInYieldProvidersTotal) external {
    _getYieldManagerStorage().userFundsInYieldProvidersTotal = _userFundsInYieldProvidersTotal;
  }

  function getPendingPermissionlessUnstake() external view returns (uint256) {
    return _getYieldManagerStorage().pendingPermissionlessUnstake;
  }

  function setPendingPermissionlessUnstake(uint256 _pendingPermissionlessUnstake) external {
    _getYieldManagerStorage().pendingPermissionlessUnstake = _pendingPermissionlessUnstake;
  }

  function decrementPendingPermissionlessUnstake(uint256 _amount) external {
    _decrementPendingPermissionlessUnstake(_amount);
  }

  function getYieldProviders() external view returns (address[] memory) {
    return _getYieldManagerStorage().yieldProviders;
  }

  function getIsL2YieldRecipientKnown(address _l2YieldRecipient) external view returns (bool) {
    return _getYieldManagerStorage().isL2YieldRecipientKnown[_l2YieldRecipient];
  }

  function setIsL2YieldRecipientKnown(address _l2YieldRecipient, bool _isKnown) external {
    _getYieldManagerStorage().isL2YieldRecipientKnown[_l2YieldRecipient] = _isKnown;
  }

  function getYieldProviderVendor(address _yieldProvider) external view returns (YieldProviderVendor) {
    return _getYieldProviderStorage(_yieldProvider).yieldProviderVendor;
  }

  function setYieldProviderVendor(address _yieldProvider, YieldProviderVendor _yieldProviderVendor) external {
    _getYieldProviderStorage(_yieldProvider).yieldProviderVendor = _yieldProviderVendor;
  }

  function getYieldProviderIsStakingPaused(address _yieldProvider) external view returns (bool) {
    return _getYieldProviderStorage(_yieldProvider).isStakingPaused;
  }

  function setYieldProviderIsStakingPaused(address _yieldProvider, bool _isPaused) external {
    _getYieldProviderStorage(_yieldProvider).isStakingPaused = _isPaused;
  }

  function getYieldProviderIsOssificationInitiated(address _yieldProvider) external view returns (bool) {
    return _getYieldProviderStorage(_yieldProvider).isOssificationInitiated;
  }

  function setYieldProviderIsOssificationInitiated(address _yieldProvider, bool _isInitiated) external {
    _getYieldProviderStorage(_yieldProvider).isOssificationInitiated = _isInitiated;
  }

  function getYieldProviderIsOssified(address _yieldProvider) external view returns (bool) {
    return _getYieldProviderStorage(_yieldProvider).isOssified;
  }

  function setYieldProviderIsOssified(address _yieldProvider, bool _isOssified) external {
    _getYieldProviderStorage(_yieldProvider).isOssified = _isOssified;
  }

  function getYieldProviderPrimaryEntrypoint(address _yieldProvider) external view returns (address) {
    return _getYieldProviderStorage(_yieldProvider).primaryEntrypoint;
  }

  function setYieldProviderPrimaryEntrypoint(address _yieldProvider, address _primaryEntrypoint) external {
    _getYieldProviderStorage(_yieldProvider).primaryEntrypoint = _primaryEntrypoint;
  }

  function getYieldProviderOssifiedEntrypoint(address _yieldProvider) external view returns (address) {
    return _getYieldProviderStorage(_yieldProvider).ossifiedEntrypoint;
  }

  function setYieldProviderOssifiedEntrypoint(address _yieldProvider, address _ossifiedEntrypoint) external {
    _getYieldProviderStorage(_yieldProvider).ossifiedEntrypoint = _ossifiedEntrypoint;
  }

  function getYieldProviderIndex(address _yieldProvider) external view returns (uint96) {
    return _getYieldProviderStorage(_yieldProvider).yieldProviderIndex;
  }

  function setYieldProviderIndex(address _yieldProvider, uint96 _yieldProviderIndex) external {
    _getYieldProviderStorage(_yieldProvider).yieldProviderIndex = _yieldProviderIndex;
  }

  function getYieldProviderUserFunds(address _yieldProvider) external view returns (uint256) {
    return _getYieldProviderStorage(_yieldProvider).userFunds;
  }

  function setYieldProviderUserFunds(address _yieldProvider, uint256 _userFunds) external {
    _getYieldProviderStorage(_yieldProvider).userFunds = _userFunds;
  }

  function getYieldProviderYieldReportedCumulative(address _yieldProvider) external view returns (uint256) {
    return _getYieldProviderStorage(_yieldProvider).yieldReportedCumulative;
  }

  function setYieldProviderYieldReportedCumulative(address _yieldProvider, uint256 _yieldReportedCumulative) external {
    _getYieldProviderStorage(_yieldProvider).yieldReportedCumulative = _yieldReportedCumulative;
  }

  function getYieldProviderLstLiabilityPrincipal(address _yieldProvider) external view returns (uint256) {
    return _getYieldProviderStorage(_yieldProvider).lstLiabilityPrincipal;
  }

  function setYieldProviderLstLiabilityPrincipal(address _yieldProvider, uint256 _lstLiabilityPrincipal) external {
    _getYieldProviderStorage(_yieldProvider).lstLiabilityPrincipal = _lstLiabilityPrincipal;
  }

  function getYieldProviderLastReportedNegativeYield(address _yieldProvider) external view returns (uint256) {
    return _getYieldProviderStorage(_yieldProvider).lastReportedNegativeYield;
  }

  function setYieldProviderLastReportedNegativeYield(address _yieldProvider, uint256 _val) external {
    _getYieldProviderStorage(_yieldProvider).lastReportedNegativeYield = _val;
  }

  function delegatecallWithdrawFromYieldProvider(address _yieldProvider, uint256 _amount) external {
    _delegatecallWithdrawFromYieldProvider(_yieldProvider, _amount);
  }

  function getEntrypointContract(address _yieldProvider) external returns (address) {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(TestLidoStVaultYieldProvider.getEntrypointContract, (_yieldProvider))
    );
    return abi.decode(data, (address));
  }

  function getDashboard(address _yieldProvider) external returns (address) {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(TestLidoStVaultYieldProvider.getDashboard, (_yieldProvider))
    );
    return abi.decode(data, (address));
  }

  function getVault(address _yieldProvider) external returns (address) {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(TestLidoStVaultYieldProvider.getVault, (_yieldProvider))
    );
    return abi.decode(data, (address));
  }

  function syncExternalLiabilitySettlement(
    address _yieldProvider,
    uint256 _liabilityShares,
    uint256 _lstLiabilityPrincipalCached
  ) external returns (uint256) {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(
        TestLidoStVaultYieldProvider.syncExternalLiabilitySettlement,
        (_yieldProvider, _liabilityShares, _lstLiabilityPrincipalCached)
      )
    );
    return abi.decode(data, (uint256));
  }

  function syncLSTLiabilityPrincipal(address _yieldProvider) external {
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.syncLSTLiabilityPrincipal, (_yieldProvider))
    );
  }

  function unstakeHarness(
    address _yieldProvider,
    bytes memory _pubkeys,
    uint64[] memory _amounts,
    address _refundRecipient
  ) external payable {
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(
        TestLidoStVaultYieldProvider.unstakeHarness,
        (_yieldProvider, _pubkeys, _amounts, _refundRecipient)
      )
    );
  }

  function validateUnstakePermissionlessRequestHarness(
    address _yieldProvider,
    uint256 _requiredUnstakeAmount,
    bytes memory _pubkeys,
    uint256 _validatorIndex,
    uint256 _slot,
    bytes memory _withdrawalParamsProof
  ) external returns (uint256) {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(
        TestLidoStVaultYieldProvider.validateUnstakePermissionlessRequestHarness,
        (
          _yieldProvider,
          _requiredUnstakeAmount,
          _pubkeys,
          uint64(_validatorIndex),
          uint64(_slot),
          _withdrawalParamsProof
        )
      )
    );
    return abi.decode(data, (uint256));
  }

  function payMaximumPossibleLSTLiability(address _yieldProvider) external {
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(TestLidoStVaultYieldProvider.payMaximumPossibleLSTLiability, (_yieldProvider))
    );
  }

  function pauseStakingIfNotAlready(address _yieldProvider) external {
    _pauseStakingIfNotAlready(_yieldProvider);
  }

  function initializeVendorContracts(
    address _yieldProvider,
    bytes memory _vendorInitializationData
  ) external returns (YieldProviderRegistration memory registrationData) {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.initializeVendorContracts, (_vendorInitializationData))
    );
    return abi.decode(data, (YieldProviderRegistration));
  }

  function exitVendorContracts(address _yieldProvider, bytes memory _vendorInitializationData) external {
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.exitVendorContracts, (_yieldProvider, _vendorInitializationData))
    );
  }

  /// @notice Emitted when a permissionless beacon chain withdrawal is requested.
  /// @param yieldProvider The yield provider address.
  /// @param stakingVault The staking vault address.
  /// @param refundRecipient Address designated to receive surplus withdrawal-fee refunds.
  /// @param maxUnstakeAmount Maximum ETH expected to be withdrawn for the request.
  /// @param pubkeys Concatenated validator pubkeys.
  /// @param amounts Withdrawal request amount array (currently length 1).
  event LidoVaultUnstakePermissionlessRequest(
    address indexed yieldProvider,
    address indexed stakingVault,
    address indexed refundRecipient,
    uint256 maxUnstakeAmount,
    bytes pubkeys,
    uint64[] amounts
  );

  /**
   * @notice Emitted when LST Liability Principal is synchronized with an external data source.
   * @param yieldProviderVendor Specific type of YieldProvider adaptor.
   * @param yieldProviderIndex Index of the YieldProvider.
   * @param oldLSTLiabilityPrincipal Old value of lstLiabilityPrincipal.
   * @param newLSTLiabilityPrincipal New value of lstLiabilityPrincipal.
   */
  event LSTLiabilityPrincipalSynced(
    YieldProviderVendor indexed yieldProviderVendor,
    uint96 indexed yieldProviderIndex,
    uint256 oldLSTLiabilityPrincipal,
    uint256 newLSTLiabilityPrincipal
  );
}
