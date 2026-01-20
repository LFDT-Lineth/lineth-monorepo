// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.33;

import { YieldManagerStorageLayout } from "./YieldManagerStorageLayout.sol";
import { IYieldManager } from "./interfaces/IYieldManager.sol";
import { IYieldProvider } from "./interfaces/IYieldProvider.sol";
import { ILineaRollupYieldExtension } from "./interfaces/ILineaRollupYieldExtension.sol";
import { YieldManagerPauseManager } from "../security/pausing/YieldManagerPauseManager.sol";
import { Math256 } from "../libraries/Math256.sol";
import { ErrorUtils } from "../libraries/ErrorUtils.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { PermissionsManager } from "../security/access/PermissionsManager.sol";
import { ProgressOssificationResult, YieldProviderRegistration } from "./interfaces/YieldTypes.sol";
import { TransientStorageReentrancyGuardUpgradeable } from "../security/reentrancy/TransientStorageReentrancyGuardUpgradeable.sol";

/**
 * @title Contract to handle native yield operations.
 * @author Consensys Software Inc.
 * @dev Sole writer to YieldManagerStorageLayout.
 * @custom:security-contact security-report@linea.build
 */
contract YieldManager is
  AccessControlUpgradeable,
  YieldManagerPauseManager,
  PermissionsManager,
  YieldManagerStorageLayout,
  TransientStorageReentrancyGuardUpgradeable,
  IYieldManager
{
  /// @notice The role required to send ETH to a yield provider.
  bytes32 public constant YIELD_PROVIDER_STAKING_ROLE = keccak256("YIELD_PROVIDER_STAKING_ROLE");

  /// @notice The role required to unstake ETH from a yield provider.
  bytes32 public constant YIELD_PROVIDER_UNSTAKER_ROLE = keccak256("YIELD_PROVIDER_UNSTAKER_ROLE");

  /// @notice The role required to request a yield report.
  bytes32 public constant YIELD_REPORTER_ROLE = keccak256("YIELD_REPORTER_ROLE");

  /// @notice The role required to pause and unpause beacon chain staking for yield provider/s that support this operation.
  bytes32 public constant STAKING_PAUSE_CONTROLLER_ROLE = keccak256("STAKING_PAUSE_CONTROLLER_ROLE");

  /// @notice The role required to initiate ossification.
  bytes32 public constant OSSIFICATION_INITIATOR_ROLE = keccak256("OSSIFICATION_INITIATOR_ROLE");

  /// @notice The role required to initiate ossification.
  bytes32 public constant OSSIFICATION_PROCESSOR_ROLE = keccak256("OSSIFICATION_PROCESSOR_ROLE");

  /// @notice The role required to set withdrawal reserve parameters.
  bytes32 public constant WITHDRAWAL_RESERVE_SETTER_ROLE = keccak256("WITHDRAWAL_RESERVE_SETTER_ROLE");

  /// @notice The role required to add and remove yield providers.
  bytes32 public constant SET_YIELD_PROVIDER_ROLE = keccak256("SET_YIELD_PROVIDER_ROLE");

  /// @notice The role required to add and remove L2 yield recipients.
  bytes32 public constant SET_L2_YIELD_RECIPIENT_ROLE = keccak256("SET_L2_YIELD_RECIPIENT_ROLE");

  /// @notice 100% in BPS.
  uint256 internal constant MAX_BPS = 10000;

  /// @notice The number of slots per historical beacon chain root.
  uint64 internal constant SLOTS_PER_HISTORICAL_ROOT = 8192;

  /**
   * @notice Minimum withdrawal reserve percentage in bps.
   * @return The withdrawal reserve percentage threshold in basis points (where 10000 = 100%).
   */
  function minimumWithdrawalReservePercentageBps() public view returns (uint256) {
    return _getYieldManagerStorage().minimumWithdrawalReservePercentageBps;
  }

  /**
   * @notice Minimum withdrawal reserve by absolute amount.
   * @return The withdrawal reserve amount threshold in wei.
   */
  function minimumWithdrawalReserveAmount() public view returns (uint256) {
    return _getYieldManagerStorage().minimumWithdrawalReserveAmount;
  }

  /**
   * @notice Target withdrawal reserve percentage in bps.
   * @return The withdrawal reserve percentage threshold in basis points (where 10000 = 100%).
   */
  function targetWithdrawalReservePercentageBps() public view returns (uint256) {
    return _getYieldManagerStorage().targetWithdrawalReservePercentageBps;
  }

  /**
   * @notice Target withdrawal reserve by absolute amount.
   * @return The withdrawal reserve amount threshold in wei.
   */
  function targetWithdrawalReserveAmount() public view returns (uint256) {
    return _getYieldManagerStorage().targetWithdrawalReserveAmount;
  }

  /// @dev Reverts if caller is not the L1MessageService.
  modifier onlyL1MessageService() {
    if (msg.sender != L1_MESSAGE_SERVICE) revert SenderNotL1MessageService();
    _;
  }

  /// @dev Reverts if the withdrawal reserve is in deficit.
  modifier onlyWhenWithdrawalReserveHealthy() {
    if (isWithdrawalReserveBelowMinimum()) revert InsufficientWithdrawalReserve();
    _;
  }

  /// @dev Reverts if the withdrawal reserve is not in deficit.
  /// @param _amountToSubtract Optional amount to subtract from the total system balance calculation.
  modifier onlyWhenWithdrawalReserveInDeficit(uint256 _amountToSubtract) {
    if (!_isWithdrawalReserveBelowMinimum(_amountToSubtract)) revert WithdrawalReserveNotInDeficit();
    _;
  }

  /// @dev Reverts if the YieldProvider address is not known.
  modifier onlyKnownYieldProvider(address _yieldProvider) {
    if (!isYieldProviderKnown(_yieldProvider)) revert UnknownYieldProvider();
    _;
  }

  /// @dev Reverts if the L2YieldRecipient address is not known.
  modifier onlyKnownL2YieldRecipient(address _l2YieldRecipient) {
    if (!_getYieldManagerStorage().isL2YieldRecipientKnown[_l2YieldRecipient]) {
      revert UnknownL2YieldRecipient();
    }
    _;
  }

  /**
   * @notice Constructs the YieldManager contract.
   * @param _l1MessageService The Linea L1MessageService address, also the withdrawal reserve holding contract.
   */
  constructor(address _l1MessageService) {
    ErrorUtils.revertIfZeroAddress(_l1MessageService);
    L1_MESSAGE_SERVICE = _l1MessageService;
    emit YieldManagerDeployed(_l1MessageService);
    _disableInitializers();
  }

  /**
   * @notice Initialise the YieldManager with reserve targets, role assignments, and allow-listed recipients.
   * @dev The supplied configuration mirrors the deployment flow described in `tmp/native-yield.md` and ensures
   *      pause roles, permissions, and withdrawal reserve thresholds are in place before any yield operations occur.
   * @param _initializationData Struct bundling pause/unpause roles, permissions, reserve targets, and default recipients.
   */
  function initialize(YieldManagerInitializationData calldata _initializationData) external initializer {
    ErrorUtils.revertIfZeroAddress(_initializationData.defaultAdmin);
    __PauseManager_init(_initializationData.pauseTypeRoles, _initializationData.unpauseTypeRoles);
    _grantRole(DEFAULT_ADMIN_ROLE, _initializationData.defaultAdmin);
    __Permissions_init(_initializationData.roleAddresses);

    _setWithdrawalReserveParameters(
      UpdateReserveParametersConfig({
        minimumWithdrawalReservePercentageBps: _initializationData.initialMinimumWithdrawalReservePercentageBps,
        minimumWithdrawalReserveAmount: _initializationData.initialMinimumWithdrawalReserveAmount,
        targetWithdrawalReservePercentageBps: _initializationData.initialTargetWithdrawalReservePercentageBps,
        targetWithdrawalReserveAmount: _initializationData.initialTargetWithdrawalReserveAmount
      })
    );

    YieldManagerStorage storage $ = _getYieldManagerStorage();
    for (uint256 i; i < _initializationData.initialL2YieldRecipients.length; i++) {
      address l2YieldRecipient = _initializationData.initialL2YieldRecipients[i];
      ErrorUtils.revertIfZeroAddress(l2YieldRecipient);
      $.isL2YieldRecipientKnown[l2YieldRecipient] = true;
    }
    // Ensure address(0) at index=0.
    $.yieldProviders.push(address(0));

    emit YieldManagerInitialized(_initializationData.initialL2YieldRecipients);
  }

  /**
   * @notice Returns the total ETH in the native yield system.
   * @dev Sums the withdrawal reserve, YieldManager balance, and capital deployed into yield providers.
   * @return totalSystemBalance Total system balance in wei.
   */
  function getTotalSystemBalance() external view returns (uint256 totalSystemBalance) {
    (totalSystemBalance, ) = _getTotalSystemBalance(0);
  }

  /**
   * @notice Returns the total ETH in the native yield system.
   * @dev Sums the withdrawal reserve, YieldManager balance, and capital deployed into yield providers.
   * @param _amountToSubtract Optional amount to subtract from the total system balance (e.g., msg.value for payable functions that withdraw funds).
   * @return totalSystemBalance Total system balance in wei.
   * @return cachedL1MessageServiceBalance Cached L1MessageService balance to avoid duplicated SLOAD + BALANCE opcodes.
   */
  function _getTotalSystemBalance(
    uint256 _amountToSubtract
  ) internal view returns (uint256 totalSystemBalance, uint256 cachedL1MessageServiceBalance) {
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    cachedL1MessageServiceBalance = L1_MESSAGE_SERVICE.balance;
    totalSystemBalance =
      cachedL1MessageServiceBalance +
      address(this).balance +
      $.userFundsInYieldProvidersTotal -
      _amountToSubtract;
  }

  /**
   * @notice Returns the effective minimum withdrawal reserve considering both percentage and absolute amount configurations.
   * @return minimumWithdrawalReserve Effective minimum reserve in wei.
   */
  function getEffectiveMinimumWithdrawalReserve() external view returns (uint256 minimumWithdrawalReserve) {
    (minimumWithdrawalReserve, ) = _getEffectiveMinimumWithdrawalReserve(0);
  }

  /**
   * @notice Returns the effective minimum withdrawal reserve considering both percentage and absolute amount configurations.
   * @param _amountToSubtract Optional amount to subtract from the total system balance calculation.
   * @return minimumWithdrawalReserve Effective minimum reserve in wei.
   * @return cachedL1MessageServiceBalance Cached L1MessageService balance to avoid duplicated SLOAD + BALANCE opcodes.
   */
  function _getEffectiveMinimumWithdrawalReserve(
    uint256 _amountToSubtract
  ) internal view returns (uint256 minimumWithdrawalReserve, uint256 cachedL1MessageServiceBalance) {
    uint256 totalSystemBalance;
    (totalSystemBalance, cachedL1MessageServiceBalance) = _getTotalSystemBalance(_amountToSubtract);
    // Get minimumWithdrawalReserveByPercentage
    uint256 minimumWithdrawalReserveByPercentage = (totalSystemBalance *
      _getYieldManagerStorage().minimumWithdrawalReservePercentageBps) / MAX_BPS;
    // Get minimumWithdrawalReserve
    minimumWithdrawalReserve = Math256.max(
      minimumWithdrawalReserveByPercentage,
      _getYieldManagerStorage().minimumWithdrawalReserveAmount
    );
  }

  /**
   * @notice Returns the effective target withdrawal reserve considering both percentage and absolute amount configurations.
   * @return targetWithdrawalReserve Effective target reserve in wei.
   */
  function getEffectiveTargetWithdrawalReserve() external view returns (uint256 targetWithdrawalReserve) {
    (targetWithdrawalReserve, ) = _getEffectiveTargetWithdrawalReserve(0);
  }

  /**
   * @notice Returns the effective target withdrawal reserve considering both percentage and absolute amount configurations.
   * @param _amountToSubtract Optional amount to subtract from the total system balance calculation.
   * @return targetWithdrawalReserve Effective target reserve in wei.
   * @return cachedL1MessageServiceBalance Cached L1MessageService balance to avoid duplicated SLOAD + BALANCE opcodes.
   */
  function _getEffectiveTargetWithdrawalReserve(
    uint256 _amountToSubtract
  ) internal view returns (uint256 targetWithdrawalReserve, uint256 cachedL1MessageServiceBalance) {
    uint256 totalSystemBalance;
    (totalSystemBalance, cachedL1MessageServiceBalance) = _getTotalSystemBalance(_amountToSubtract);
    uint256 targetWithdrawalReserveByPercentage = (totalSystemBalance *
      _getYieldManagerStorage().targetWithdrawalReservePercentageBps) / MAX_BPS;
    targetWithdrawalReserve = Math256.max(
      targetWithdrawalReserveByPercentage,
      _getYieldManagerStorage().targetWithdrawalReserveAmount
    );
  }

  /**
   * @notice Returns the shortfall between the minimum reserve threshold and the current reserve balance.
   * @return minimumReserveDeficit Amount of ETH required to meet the minimum reserve, or zero if already satisfied.
   */
  function getMinimumReserveDeficit() public view returns (uint256 minimumReserveDeficit) {
    (uint256 minimumWithdrawalReserve, uint256 cachedL1MessageServiceBalance) = _getEffectiveMinimumWithdrawalReserve(
      0
    );
    minimumReserveDeficit = Math256.safeSub(minimumWithdrawalReserve, cachedL1MessageServiceBalance);
  }

  /**
   * @notice Returns the shortfall between the target reserve threshold and the current reserve balance.
   * @return targetReserveDeficit Amount of ETH required to meet the target reserve, or zero if already satisfied.
   */
  function getTargetReserveDeficit() external view returns (uint256 targetReserveDeficit) {
    return _getTargetReserveDeficit(0);
  }

  /**
   * @notice Returns the shortfall between the target reserve threshold and the current reserve balance.
   * @param _amountToSubtract Optional amount to subtract from the total system balance calculation.
   * @return targetReserveDeficit Amount of ETH required to meet the target reserve, or zero if already satisfied.
   */
  function _getTargetReserveDeficit(uint256 _amountToSubtract) internal view returns (uint256 targetReserveDeficit) {
    (uint256 targetWithdrawalReserve, uint256 cachedL1MessageServiceBalance) = _getEffectiveTargetWithdrawalReserve(
      _amountToSubtract
    );
    targetReserveDeficit = Math256.safeSub(targetWithdrawalReserve, cachedL1MessageServiceBalance);
  }

  /**
   * @return bool True if the withdrawal reserve balance is below the effective minimum threshold.
   */
  function isWithdrawalReserveBelowMinimum() public view returns (bool) {
    return _isWithdrawalReserveBelowMinimum(0);
  }

  /**
   * @notice Internal function to check if the withdrawal reserve balance is below the effective minimum threshold.
   * @param _amountToSubtract Optional amount to subtract from the total system balance calculation.
   * @return bool True if the withdrawal reserve balance is below the effective minimum threshold.
   */
  function _isWithdrawalReserveBelowMinimum(uint256 _amountToSubtract) internal view returns (bool) {
    (uint256 minimumWithdrawalReserve, uint256 cachedL1MessageServiceBalance) = _getEffectiveMinimumWithdrawalReserve(
      _amountToSubtract
    );
    return cachedL1MessageServiceBalance < minimumWithdrawalReserve;
  }

  /**
   * @param _l2YieldRecipient The L2YieldRecipient address.
   * @return bool True if the L2YieldRecipient is on the allowlist.
   */
  function isL2YieldRecipientKnown(address _l2YieldRecipient) external view returns (bool) {
    return _getYieldManagerStorage().isL2YieldRecipientKnown[_l2YieldRecipient];
  }

  /**
   * @param _yieldProvider The YieldProvider address.
   * @return bool True if the YieldProvider is registered.
   */
  function isYieldProviderKnown(address _yieldProvider) public view returns (bool) {
    return _getYieldProviderStorage(_yieldProvider).yieldProviderIndex != 0;
  }

  /**
   * @notice Returns the number of registered yield provider adaptor contracts.
   * @return count Total number of yield providers tracked by the YieldManager.
   */
  function yieldProviderCount() external view override returns (uint256 count) {
    count = _getYieldManagerStorage().yieldProviders.length - 1;
  }

  /**
   * @notice Returns the yield provider address stored at a specific index in the registry.
   * @dev Uses 1-based indexing: 1 returns the first element.
   * @dev 0 index is the sentinel value for a yield provider not being registered.
   * @param _index Index of the yield provider to query.
   * @return yieldProvider Yield provider adaptor address stored at the supplied index.
   *                       - Zero address if yield provider not registered.
   */
  function yieldProviderByIndex(uint256 _index) external view override returns (address yieldProvider) {
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    uint256 length = $.yieldProviders.length;
    if (_index == 0 || _index >= length) {
      revert YieldProviderIndexOutOfBounds(_index, Math256.safeSub(length, 1));
    }
    yieldProvider = $.yieldProviders[_index];
  }

  /**
   * @notice Returns the full state for a registered yield provider.
   * @param _yieldProvider Yield provider adaptor address to query.
   * @return yieldProviderData Struct containing the yield provider data.
   */
  function getYieldProviderData(
    address _yieldProvider
  )
    external
    view
    override
    onlyKnownYieldProvider(_yieldProvider)
    returns (YieldManagerStorageLayout.YieldProviderStorage memory yieldProviderData)
  {
    YieldProviderStorage storage $$ = _getYieldProviderStorage(_yieldProvider);
    yieldProviderData = YieldManagerStorageLayout.YieldProviderStorage({
      yieldProviderVendor: $$.yieldProviderVendor,
      isStakingPaused: $$.isStakingPaused,
      isOssificationInitiated: $$.isOssificationInitiated,
      isOssified: $$.isOssified,
      primaryEntrypoint: $$.primaryEntrypoint,
      ossifiedEntrypoint: $$.ossifiedEntrypoint,
      yieldProviderIndex: $$.yieldProviderIndex,
      userFunds: $$.userFunds,
      yieldReportedCumulative: $$.yieldReportedCumulative,
      lstLiabilityPrincipal: $$.lstLiabilityPrincipal,
      lastReportedNegativeYield: $$.lastReportedNegativeYield
    });
  }

  /**
   * @notice Returns the tracked user funds for a specific yield provider.
   * @param _yieldProvider Yield provider adaptor address to query.
   * @return funds Amount of user funds currently attributed to the yield provider.
   */
  function userFunds(
    address _yieldProvider
  ) external view override onlyKnownYieldProvider(_yieldProvider) returns (uint256 funds) {
    funds = _getYieldProviderStorage(_yieldProvider).userFunds;
  }

  /**
   * @notice Returns whether staking is currently paused for a specific yield provider.
   * @param _yieldProvider Yield provider adaptor address to query.
   * @return isPaused True if staking is paused on the yield provider.
   */
  function isStakingPaused(
    address _yieldProvider
  ) external view override onlyKnownYieldProvider(_yieldProvider) returns (bool isPaused) {
    isPaused = _getYieldProviderStorage(_yieldProvider).isStakingPaused;
  }

  /**
   * @notice Returns whether a yield provider has been fully ossified.
   * @param _yieldProvider Yield provider adaptor address to query.
   * @return bool True if the ossification process has completed for the yield provider.
   */
  function isOssified(
    address _yieldProvider
  ) external view override onlyKnownYieldProvider(_yieldProvider) returns (bool) {
    return _getYieldProviderStorage(_yieldProvider).isOssified;
  }

  /**
   * @notice Returns whether a yield provider has initiated ossification.
   * @param _yieldProvider Yield provider adaptor address to query.
   * @return isInitiated True if ossification has been initiated for the yield provider.
   */
  function isOssificationInitiated(
    address _yieldProvider
  ) external view override onlyKnownYieldProvider(_yieldProvider) returns (bool isInitiated) {
    isInitiated = _getYieldProviderStorage(_yieldProvider).isOssificationInitiated;
  }

  /**
   * @notice Returns the aggregate user funds deployed across all registered yield providers.
   * @return totalUserFunds Aggregate amount of user funds currently deployed across yield providers.
   */
  function userFundsInYieldProvidersTotal() external view override returns (uint256 totalUserFunds) {
    totalUserFunds = _getYieldManagerStorage().userFundsInYieldProvidersTotal;
  }

  /**
   * @notice Returns the amount of ETH expected from pending permissionless unstake requests.
   * @return pendingUnstake Amount of ETH pending arrival from the beacon chain via permissionless unstaking.
   */
  function pendingPermissionlessUnstake() external view override returns (uint256 pendingUnstake) {
    pendingUnstake = _getYieldManagerStorage().pendingPermissionlessUnstake;
  }

  /**
   * @notice Returns the last proven slot for a validator index.
   * @param _validatorIndex The validator index to query.
   * @return lastProvenSlot The last proven slot for the validator index, or 0 if no slot has been proven yet.
   */
  function lastProvenSlot(uint64 _validatorIndex) external view override returns (uint64 lastProvenSlot) {
    lastProvenSlot = _getYieldManagerStorage().lastProvenSlot[_validatorIndex];
  }

  /**
   * @notice Helper function to delegatecall YieldProvider adaptor instances.
   * @param _yieldProvider The yield provider address.
   * @param _callData Calldata to send with YieldProvider delegatecall.
   * @return Return data from YieldProvider delegatecall.
   */
  function _delegatecallYieldProvider(address _yieldProvider, bytes memory _callData) internal returns (bytes memory) {
    (bool success, bytes memory returnData) = _yieldProvider.delegatecall(_callData);
    if (!success) {
      if (returnData.length > 0) {
        /// @solidity memory-safe-assembly
        assembly {
          revert(add(returnData, 32), mload(returnData))
        }
      }
      revert DelegateCallFailed();
    }
    return returnData;
  }

  /**
   * @notice Helper function to send ETH to the Linea L1MessageService (i.e. withdrawal reserve).
   * @param _amount Amount of ETH to send.
   */
  function _fundReserve(uint256 _amount) internal virtual {
    ILineaRollupYieldExtension(L1_MESSAGE_SERVICE).fund{ value: _amount }();
  }

  /**
   * @param _yieldProvider The yield provider address.
   * @return withdrawableAmount Amount of ETH that can be instantly withdrawn from the YieldProvider.
   */
  function withdrawableValue(
    address _yieldProvider
  ) public onlyKnownYieldProvider(_yieldProvider) returns (uint256 withdrawableAmount) {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.withdrawableValue, (_yieldProvider))
    );
    uint256 fetchedWithdrawableAmount = abi.decode(data, (uint256));
    // We tolerate userFunds > withdrawableValue, as this means we have incurred negative yield. We assume it is a temporary situation that is fixed with incoming yield.
    // We don't tolerate the reverse, because it means existing funds on the YieldProvider are unavailable for withdrawal.
    withdrawableAmount = Math256.min(fetchedWithdrawableAmount, _getYieldProviderStorage(_yieldProvider).userFunds);
  }

  /**
   * @notice Receive ETH from the withdrawal reserve.
   * @dev Only accepts calls from the withdrawal reserve.
   * @dev It is possible for an arbitrary user to call this via `L1.claimMessage()`,
   *    this results in a user effectively donating their ETH balance to YieldManager.
   *    This does not violate the safety property of user principal protection, as the user has forfeited their principal.
   * @dev Reverts if, after transfer, the withdrawal reserve will be below the minimum threshold.
   */
  function receiveFundsFromReserve() external payable onlyL1MessageService onlyWhenWithdrawalReserveHealthy {
    emit ReserveFundsReceived(msg.value);
  }

  /**
   * @notice Send ETH to the L1MessageService.
   * @dev YIELD_PROVIDER_UNSTAKER_ROLE is required to execute.
   * @param _amount        The amount of ETH to send.
   */
  function transferFundsToReserve(
    uint256 _amount
  ) external whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_UNSTAKING) onlyRole(YIELD_PROVIDER_UNSTAKER_ROLE) {
    _fundReserve(_amount);
    // Destination will emit the event.
  }

  /**
   * @notice Send ETH to the specified YieldProvider instance.
   * @dev YIELD_PROVIDER_STAKING_ROLE is required to execute.
   * @dev Reverts if the withdrawal reserve is below the minimum threshold.
   * @param _yieldProvider The target yield provider contract.
   * @param _amount        The amount of ETH to send.
   */
  function fundYieldProvider(
    address _yieldProvider,
    uint256 _amount
  )
    external
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_STAKING)
    onlyKnownYieldProvider(_yieldProvider)
    onlyRole(YIELD_PROVIDER_STAKING_ROLE)
    onlyWhenWithdrawalReserveHealthy
  {
    if (_getTargetReserveDeficit(0) > 0) _pauseStakingIfNotAlready(_yieldProvider);
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.fundYieldProvider, (_yieldProvider, _amount))
    );
    _getYieldManagerStorage().userFundsInYieldProvidersTotal += _amount;
    _getYieldProviderStorage(_yieldProvider).userFunds += _amount;
    emit YieldProviderFunded(_yieldProvider, _amount);
  }

  /**
   * @notice Report newly accrued yield for the YieldProvider since the last report.
   * @dev YIELD_REPORTER_ROLE is required to execute.
   * @dev Reported yield excludes amounts reserved to pay system obligations.
   * @param _yieldProvider      The yield provider address.
   * @param _l2YieldRecipient   The L2YieldRecipient address.
   * @return newReportedYield New net yield (denominated in ETH) since the prior report.
   * @return outstandingNegativeYield Amount of outstanding negative yield.
   */
  function reportYield(
    address _yieldProvider,
    address _l2YieldRecipient
  )
    external
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_REPORTING)
    onlyKnownYieldProvider(_yieldProvider)
    onlyKnownL2YieldRecipient(_l2YieldRecipient)
    onlyRole(YIELD_REPORTER_ROLE)
    returns (uint256 newReportedYield, uint256 outstandingNegativeYield)
  {
    bytes memory data = _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.reportYield, (_yieldProvider))
    );
    (newReportedYield, outstandingNegativeYield) = abi.decode(data, (uint256, uint256));
    YieldProviderStorage storage $$ = _getYieldProviderStorage(_yieldProvider);
    $$.userFunds += newReportedYield;
    $$.yieldReportedCumulative += newReportedYield;
    $$.lastReportedNegativeYield = outstandingNegativeYield;
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    $.userFundsInYieldProvidersTotal += newReportedYield;
    ILineaRollupYieldExtension(L1_MESSAGE_SERVICE).reportNativeYield(newReportedYield, _l2YieldRecipient);
    emit NativeYieldReported(_yieldProvider, _l2YieldRecipient, newReportedYield, outstandingNegativeYield);
  }

  /**
   * @notice Request beacon chain withdrawal from specified yield provider.
   * @dev YIELD_PROVIDER_UNSTAKER_ROLE is required to execute.
   * @param _yieldProvider      Yield provider address.
   * @param _withdrawalParams   Provider-specific withdrawal parameters.
   */
  function unstake(
    address _yieldProvider,
    bytes memory _withdrawalParams
  )
    external
    payable
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_UNSTAKING)
    onlyKnownYieldProvider(_yieldProvider)
    onlyRole(YIELD_PROVIDER_UNSTAKER_ROLE)
  {
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.unstake, (_yieldProvider, _withdrawalParams))
    );
    // Event emitted by YieldProvider which has provider-specific decoding of _withdrawalParams
  }

  /**
   * @notice Permissionlessly request beacon chain withdrawal from a specified yield provider.
   * @dev    Callable only when the withdrawal reserve is in deficit.
   * @dev    The permissionless unstake amount is capped to the remaining target deficit that
   *         cannot be covered by other liquidity sources:
   *
   *         PERMISSIONLESS_UNSTAKE_AMOUNT ≤
   *           TARGET_DEFICIT
   *         - YIELD_PROVIDER_BALANCE
   *         - YIELD_MANAGER_BALANCE
   *         - PENDING_PERMISSIONLESS_UNSTAKE
   *
   * @dev PENDING_PERMISSIONLESS_UNSTAKE will be greedily reduced with i.) donations or ii.) future withdrawals from the YieldProvider
   * @dev msg.value will not remain in the native yield system, so it must be subtracted from total system balance.
   * @param _yieldProvider          Yield provider address.
   * @param _withdrawalParams       Provider-specific withdrawal parameters.
   * @param _withdrawalParamsProof  Data containing merkle proof of _withdrawalParams to be verified against EIP-4788 beacon chain root.
   */
  function unstakePermissionless(
    address _yieldProvider,
    uint64 _validatorIndex,
    uint64 _slot,
    bytes calldata _withdrawalParams,
    bytes calldata _withdrawalParamsProof
  )
    external
    payable
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_PERMISSIONLESS_ACTIONS)
    onlyKnownYieldProvider(_yieldProvider)
    onlyWhenWithdrawalReserveInDeficit(msg.value)
    nonReentrant
  {
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    _validateAndUpdateLastProvenSlot($, _validatorIndex, _slot);

    uint256 requiredUnstakeAmountWei = Math256.safeSub(
      _getTargetReserveDeficit(msg.value),
      address(this).balance - msg.value + withdrawableValue(_yieldProvider) + $.pendingPermissionlessUnstake
    );
    if (requiredUnstakeAmountWei == 0) revert NoRequirementToUnstakePermissionless();

    uint256 unstakedAmountWei = abi.decode(
      _delegatecallYieldProvider(
        _yieldProvider,
        abi.encodeCall(
          IYieldProvider.unstakePermissionless,
          (_yieldProvider, requiredUnstakeAmountWei, _validatorIndex, _slot, _withdrawalParams, _withdrawalParamsProof)
        )
      ),
      (uint256)
    );
    if (unstakedAmountWei == 0) revert YieldProviderReturnedZeroUnstakeAmount();
    if (unstakedAmountWei > requiredUnstakeAmountWei) {
      revert UnstakedAmountExceedsRequired(unstakedAmountWei, requiredUnstakeAmountWei);
    }

    $.pendingPermissionlessUnstake += unstakedAmountWei;
    emit UnstakePermissionlessRequest(
      _yieldProvider,
      _validatorIndex,
      _slot,
      requiredUnstakeAmountWei,
      unstakedAmountWei,
      _withdrawalParams
    );
  }

  /**
   * @notice Validates that a slot is sufficiently far from the last proven slot and updates the last proven slot.
   * @dev Used to avoid replay attack and enforce minimum 8192 slot interval between proofs.
   * @dev Perform storage update before potential reentrancy points to follow checks-effects-interactions pattern.
   * @dev This function was extracted from unstakePermissionless to avoid stack-too-deep compilation errors.
   * @param $ Storage pointer for YieldManager storage.
   * @param _validatorIndex The index of the validator.
   * @param _slot The slot to validate and set as the new last proven slot.
   * @custom:reverts SlotTooCloseToLastProvenSlot If the provided slot is within SLOTS_PER_HISTORICAL_ROOT
   *         of the last proven slot for this validator.
   */
  function _validateAndUpdateLastProvenSlot(
    YieldManagerStorage storage $,
    uint64 _validatorIndex,
    uint64 _slot
  ) internal {
    uint64 lastProvenSlot = $.lastProvenSlot[_validatorIndex];
    if (_slot <= lastProvenSlot + SLOTS_PER_HISTORICAL_ROOT) {
      revert SlotTooCloseToLastProvenSlot(_validatorIndex, lastProvenSlot, _slot);
    }
    // Place storage update before possible reentrancy in _delegatecallYieldProvider
    $.lastProvenSlot[_validatorIndex] = _slot;
  }

  /**
   * @notice Safely withdraws ETH from a YieldProvider, capped by the available withdrawable amount.
   * @dev YIELD_PROVIDER_UNSTAKER_ROLE is required to execute.
   * @dev This function proactively allocates withdrawn funds in the following priority:
   *      1. If the withdrawal reserve is below the target threshold, ETH is routed to the reserve
   *      to restore the deficit.
   *      2. YieldManager will keep the remainder.
   * @param _yieldProvider The yield provider address.
   * @param _amount Amount to withdraw.
   */
  function safeWithdrawFromYieldProvider(
    address _yieldProvider,
    uint256 _amount
  )
    external
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_UNSTAKING)
    onlyKnownYieldProvider(_yieldProvider)
    onlyRole(YIELD_PROVIDER_UNSTAKER_ROLE)
  {
    // @dev Call hook before querying withdrawableValue
    _delegatecallBeforeWithdrawFromYieldProvider(_yieldProvider, false);
    _withdrawFromYieldProvider(_yieldProvider, Math256.min(withdrawableValue(_yieldProvider), _amount));
  }

  /**
   * @notice Withdraw ETH from a YieldProvider.
   * @dev YIELD_PROVIDER_UNSTAKER_ROLE is required to execute.
   * @dev This function proactively allocates withdrawn funds in the following priority:
   *      1. If the withdrawal reserve is below the target threshold, ETH is routed to the reserve
   *      to restore the deficit.
   *      2. YieldManager will keep the remainder.
   * @param _yieldProvider The yield provider address.
   * @param _amount Amount to withdraw.
   */
  function _withdrawFromYieldProvider(address _yieldProvider, uint256 _amount) internal {
    uint256 targetDeficit = _getTargetReserveDeficit(0);
    _delegatecallWithdrawFromYieldProvider(_yieldProvider, _amount);
    uint256 toReserve = Math256.min(_amount, targetDeficit);
    // Send funds to L1MessageService if targetDeficit
    if (toReserve > 0) {
      _fundReserve(toReserve);
    }
    if (targetDeficit > toReserve) _pauseStakingIfNotAlready(_yieldProvider);
    emit YieldProviderWithdrawal(_yieldProvider, _amount, toReserve);
  }

  /**
   * @notice Helper function to withdraw from a yield provider and update state accordingly.
   * @param _yieldProvider The yield provider address.
   * @param _amount Amount to withdraw.
   */
  function _delegatecallWithdrawFromYieldProvider(address _yieldProvider, uint256 _amount) internal {
    if (_amount == 0) return;
    YieldProviderStorage storage $$ = _getYieldProviderStorage(_yieldProvider);
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.withdrawFromYieldProvider, (_yieldProvider, _amount))
    );
    $$.userFunds -= _amount;
    _getYieldManagerStorage().userFundsInYieldProvidersTotal -= _amount;
  }

  /**
   * @notice Hook called before withdrawing ETH from the YieldProvider.
   * @param _yieldProvider The yield provider address.
   * @param _isPermissionlessReserveDeficitWithdrawal Whether this is a permissionless reserve deficit withdrawal.
   */
  function _delegatecallBeforeWithdrawFromYieldProvider(
    address _yieldProvider,
    bool _isPermissionlessReserveDeficitWithdrawal
  ) internal {
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(
        IYieldProvider.beforeWithdrawFromYieldProvider,
        (_yieldProvider, _isPermissionlessReserveDeficitWithdrawal)
      )
    );
  }

  /**
   * @notice Helper function to decrement the pending permissionless unstake amount.
   * @param _amount Amount to decrement from pending permissionless unstake.
   */
  function _decrementPendingPermissionlessUnstake(uint256 _amount) internal {
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    uint256 pendingPermissionlessUnstakeAmount = $.pendingPermissionlessUnstake;
    $.pendingPermissionlessUnstake = Math256.safeSub(pendingPermissionlessUnstakeAmount, _amount);
  }

  /**
   * @notice Safely rebalance ETH from the YieldManager and specified yield provider, sending it to the L1MessageService.
   * @dev Caps the rebalance amount to the provider's current withdrawable value plus the YieldManager's balance.
   *      This is to mitigate frontrunning that depletes the withdrawable value,
   *      which would result in revert of the regular `addToWithdrawalReserve` function.
   * @dev YIELD_PROVIDER_UNSTAKER_ROLE is required to execute.
   * @param _yieldProvider          Yield provider address.
   * @param _amount                 Amount to rebalance from the YieldManager and specified YieldProvider.
   */
  function safeAddToWithdrawalReserve(
    address _yieldProvider,
    uint256 _amount
  )
    external
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_UNSTAKING)
    onlyKnownYieldProvider(_yieldProvider)
    onlyRole(YIELD_PROVIDER_UNSTAKER_ROLE)
  {
    // @dev Call hook before querying withdrawableValue
    _delegatecallBeforeWithdrawFromYieldProvider(_yieldProvider, false);
    _addToWithdrawalReserve(
      _yieldProvider,
      Math256.min(withdrawableValue(_yieldProvider) + address(this).balance, _amount)
    );
  }

  /**
   * @notice Helper function to rebalance ETH from the YieldManager and specified yield provider, sending it to the L1MessageService.
   * @param _yieldProvider          Yield provider address.
   * @param _amount                 Amount to rebalance from the YieldManager and specified YieldProvider.
   */
  function _addToWithdrawalReserve(address _yieldProvider, uint256 _amount) internal {
    if (_getTargetReserveDeficit(0) > _amount) _pauseStakingIfNotAlready(_yieldProvider);
    // First see if we can fully settle from YieldManager
    // @dev When using YieldManager balance directly, LST liability payment is skipped for operational efficiency
    uint256 yieldManagerBalance = address(this).balance;
    if (yieldManagerBalance >= _amount) {
      _fundReserve(_amount);
      emit WithdrawalReserveAugmented(_yieldProvider, _amount, _amount, 0);
      return;
    }

    // Insufficient balance on YieldManager, must withdraw from YieldProvider
    uint256 withdrawRequestAmount = _amount - yieldManagerBalance;
    _delegatecallWithdrawFromYieldProvider(_yieldProvider, withdrawRequestAmount);

    // Send to reserve
    _fundReserve(_amount);

    emit WithdrawalReserveAugmented(_yieldProvider, _amount, yieldManagerBalance, withdrawRequestAmount);
  }

  /**
   * @notice Permissionlessly top up the withdrawal reserve to the target threshold using available liquidity.
   * @dev Callable only when the reserve is below the effective minimum threshold.
   * @dev The function first spends the YieldManager's balance to clear the target threshold deficit.
   * @dev If a target deficit still remains, then it will withdraw from the specified YieldProvider.
   * @param _yieldProvider The yield provider address.
   */
  function replenishWithdrawalReserve(
    address _yieldProvider
  )
    external
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_PERMISSIONLESS_ACTIONS)
    onlyKnownYieldProvider(_yieldProvider)
    onlyWhenWithdrawalReserveInDeficit(0)
    nonReentrant
  {
    uint256 targetDeficit = _getTargetReserveDeficit(0);

    // First see if we can fully settle from YieldManager
    // @dev When using YieldManager balance directly, LST liability payment is skipped for operational efficiency
    uint256 yieldManagerBalance = address(this).balance;
    if (yieldManagerBalance >= targetDeficit) {
      _fundReserve(targetDeficit);
      emit WithdrawalReserveReplenished(_yieldProvider, targetDeficit, targetDeficit, targetDeficit, 0);
      return;
    }

    // @dev Call hook before querying withdrawableValue
    _delegatecallBeforeWithdrawFromYieldProvider(_yieldProvider, true);

    // Insufficient balance on YieldManager, must withdraw from YieldProvider
    uint256 yieldProviderBalance = withdrawableValue(_yieldProvider);
    if (yieldProviderBalance == 0 && yieldManagerBalance == 0) revert NoAvailableFundsToReplenishWithdrawalReserve();
    uint256 withdrawAmount = Math256.min(yieldProviderBalance, targetDeficit - yieldManagerBalance);
    _delegatecallWithdrawFromYieldProvider(_yieldProvider, withdrawAmount);
    _fundReserve(yieldManagerBalance + withdrawAmount);

    // Pause staking if remaining target deficit
    if (targetDeficit - yieldManagerBalance > yieldProviderBalance) {
      _pauseStakingIfNotAlready(_yieldProvider);
    }

    emit WithdrawalReserveReplenished(
      _yieldProvider,
      targetDeficit,
      yieldManagerBalance + withdrawAmount,
      yieldManagerBalance,
      withdrawAmount
    );
  }

  /**
   * @notice Pauses beacon chain deposits for specified yield provider.
   * @dev STAKING_PAUSE_CONTROLLER_ROLE is required to execute.
   * @param _yieldProvider The yield provider address.
   */
  function pauseStaking(
    address _yieldProvider
  ) external onlyKnownYieldProvider(_yieldProvider) onlyRole(STAKING_PAUSE_CONTROLLER_ROLE) {
    if (_getYieldProviderStorage(_yieldProvider).isStakingPaused) {
      revert StakingAlreadyPaused();
    }
    _pauseStaking(_yieldProvider);
  }

  /**
   * @notice Pauses beacon chain deposits for specified yield provider.
   * @param _yieldProvider The yield provider address.
   */
  function _pauseStaking(address _yieldProvider) internal {
    _delegatecallYieldProvider(_yieldProvider, abi.encodeCall(IYieldProvider.pauseStaking, (_yieldProvider)));
    _getYieldProviderStorage(_yieldProvider).isStakingPaused = true;
    emit YieldProviderStakingPaused(_yieldProvider);
  }

  /**
   * @notice Pauses beacon chain deposits for specified yield provider if not paused.
   * @param _yieldProvider The yield provider address.
   */
  function _pauseStakingIfNotAlready(address _yieldProvider) internal {
    if (!_getYieldProviderStorage(_yieldProvider).isStakingPaused) {
      _pauseStaking(_yieldProvider);
    }
  }

  /**
   * @notice Unpauses beacon chain deposits for specified yield provider.
   * @dev STAKING_PAUSE_CONTROLLER_ROLE is required to execute.
   * @dev Will revert if:
   *      - The withdrawal reserve is in deficit, or
   *      - There is an existing LST liability, or
   *      - Ossification has been initiated or finalized.
   * @param _yieldProvider The yield provider address.
   */
  function unpauseStaking(
    address _yieldProvider
  )
    external
    onlyKnownYieldProvider(_yieldProvider)
    onlyRole(STAKING_PAUSE_CONTROLLER_ROLE)
    onlyWhenWithdrawalReserveHealthy
  {
    // Other checks for unstaking
    YieldProviderStorage storage $$ = _getYieldProviderStorage(_yieldProvider);
    if (!$$.isStakingPaused) {
      revert StakingAlreadyUnpaused();
    }
    // Synchronize lstLiabilityPrincipal before using it for a check.
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.syncLSTLiabilityPrincipal, (_yieldProvider))
    );
    if ($$.lstLiabilityPrincipal > 0) {
      revert UnpauseStakingForbiddenWithCurrentLSTLiability();
    }
    if ($$.isOssificationInitiated && !$$.isOssified) {
      revert UnpauseStakingForbiddenDuringPendingOssification();
    }
    _unpauseStaking(_yieldProvider);
  }

  /**
   * @notice Helper function to unpauses beacon chain deposits for a specified yield provider.
   * @param _yieldProvider The yield provider address.
   */
  function _unpauseStaking(address _yieldProvider) internal {
    _delegatecallYieldProvider(_yieldProvider, abi.encodeCall(IYieldProvider.unpauseStaking, (_yieldProvider)));
    _getYieldProviderStorage(_yieldProvider).isStakingPaused = false;
    emit YieldProviderStakingUnpaused(_yieldProvider);
  }

  /**
   * @notice Withdraw LST from a specified YieldProvider instance.
   * @dev Callable only by the L1MessageService.
   * @dev Will pause staking to mitigate further reserve deficits.
   * @param _yieldProvider The yield provider address.
   * @param _amount Amount of LST (ETH-denominated) to withdraw.
   * @param _recipient L1 address to receive the LST.
   */
  function withdrawLST(
    address _yieldProvider,
    uint256 _amount,
    address _recipient
  )
    external
    whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_PERMISSIONLESS_ACTIONS)
    onlyKnownYieldProvider(_yieldProvider)
    onlyL1MessageService
  {
    if (!ILineaRollupYieldExtension(L1_MESSAGE_SERVICE).isWithdrawLSTAllowed()) {
      revert LSTWithdrawalNotAllowed();
    }
    YieldProviderStorage storage $$ = _getYieldProviderStorage(_yieldProvider);
    if (_amount + $$.lastReportedNegativeYield > $$.userFunds) {
      revert LSTWithdrawalExceedsYieldProviderFunds();
    }
    _pauseStakingIfNotAlready(_yieldProvider);
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.withdrawLST, (_yieldProvider, _amount, _recipient))
    );
    // If L1MessageService users withdraw LST, then the amount "owed to users" is decremented.
    _getYieldManagerStorage().userFundsInYieldProvidersTotal -= _amount;
    $$.userFunds -= _amount;
    $$.lstLiabilityPrincipal += _amount;
    emit LSTMinted(_yieldProvider, _recipient, _amount);
  }

  /**
   * @notice Initiate the ossification sequence for a provider.
   * @dev OSSIFICATION_INITIATOR_ROLE is required to execute.
   * @dev Will pause beacon chain staking and LST withdrawals.
   * @dev WARNING: This operation irreversibly pauses beacon chain deposits.
   * @param _yieldProvider The yield provider address.
   */
  function initiateOssification(
    address _yieldProvider
  ) external onlyKnownYieldProvider(_yieldProvider) onlyRole(OSSIFICATION_INITIATOR_ROLE) {
    YieldProviderStorage storage $$ = _getYieldProviderStorage(_yieldProvider);
    if ($$.isOssificationInitiated) {
      revert OssificationAlreadyInitiated();
    }
    if ($$.isOssified) {
      revert AlreadyOssified();
    }
    _delegatecallYieldProvider(_yieldProvider, abi.encodeCall(IYieldProvider.initiateOssification, (_yieldProvider)));
    _pauseStakingIfNotAlready(_yieldProvider);
    $$.isOssificationInitiated = true;
    emit YieldProviderOssificationInitiated(_yieldProvider);
  }

  /**
   * @notice Progress an initiated ossification process.
   * @dev OSSIFICATION_PROCESSOR_ROLE is required to execute.
   * @param _yieldProvider The yield provider address.
   * @return progressOssificationResult The operation result.
   */
  function progressPendingOssification(
    address _yieldProvider
  )
    external
    onlyKnownYieldProvider(_yieldProvider)
    onlyRole(OSSIFICATION_PROCESSOR_ROLE)
    returns (ProgressOssificationResult progressOssificationResult)
  {
    YieldProviderStorage storage $$ = _getYieldProviderStorage(_yieldProvider);
    if (!$$.isOssificationInitiated) {
      revert OssificationNotInitiated();
    }
    if ($$.isOssified) {
      revert AlreadyOssified();
    }

    progressOssificationResult = abi.decode(
      _delegatecallYieldProvider(
        _yieldProvider,
        abi.encodeCall(IYieldProvider.progressPendingOssification, (_yieldProvider))
      ),
      (ProgressOssificationResult)
    );
    if (progressOssificationResult == ProgressOssificationResult.COMPLETE) {
      $$.isOssified = true;
    }
    emit YieldProviderOssificationProcessed(_yieldProvider, progressOssificationResult);
  }

  /// @notice Function to receive a basic ETH transfer.
  /// @dev Any withdrawals from the YieldProvider will greedily decrement `pendingPermissionlessUnstake` on the assumption
  ///      that the requested withdrawal has arrived at a YieldProvider.
  receive() external payable {
    // Greedily reduce pendingPermissionlessUnstake with every donation or YieldProvider withdrawal.
    _decrementPendingPermissionlessUnstake(msg.value);
  }

  /**
   * @notice Register a new YieldProvider adaptor instance.
   * @dev SET_YIELD_PROVIDER_ROLE is required to execute.
   * @param _yieldProvider The yield provider address.
   * @param _vendorInitializationData Vendor-specific initialization data.
   */
  function addYieldProvider(
    address _yieldProvider,
    bytes memory _vendorInitializationData
  ) external onlyRole(SET_YIELD_PROVIDER_ROLE) {
    ErrorUtils.revertIfZeroAddress(_yieldProvider);
    if (isYieldProviderKnown(_yieldProvider)) revert YieldProviderAlreadyAdded();

    YieldProviderRegistration memory registrationData = abi.decode(
      _delegatecallYieldProvider(
        _yieldProvider,
        abi.encodeCall(IYieldProvider.initializeVendorContracts, (_vendorInitializationData))
      ),
      (YieldProviderRegistration)
    );

    YieldManagerStorage storage $ = _getYieldManagerStorage();
    uint96 yieldProviderIndex = uint96($.yieldProviders.length);
    $.yieldProviders.push(_yieldProvider);
    $.yieldProviderStorage[_yieldProvider] = YieldProviderStorage({
      yieldProviderVendor: registrationData.yieldProviderVendor,
      isStakingPaused: false,
      isOssificationInitiated: false,
      isOssified: false,
      primaryEntrypoint: registrationData.primaryEntrypoint,
      ossifiedEntrypoint: registrationData.ossifiedEntrypoint,
      yieldProviderIndex: yieldProviderIndex,
      userFunds: registrationData.usersFundsIncrement,
      yieldReportedCumulative: 0,
      lstLiabilityPrincipal: 0,
      lastReportedNegativeYield: 0
    });
    $.userFundsInYieldProvidersTotal += registrationData.usersFundsIncrement;
    emit YieldProviderAdded(
      _yieldProvider,
      registrationData.yieldProviderVendor,
      registrationData.primaryEntrypoint,
      registrationData.ossifiedEntrypoint,
      registrationData.usersFundsIncrement
    );
  }

  /**
   * @notice Remove a YieldProvider instance from the YieldManager.
   * @dev SET_YIELD_PROVIDER_ROLE is required to execute.
   * @dev Has safety checks to ensure that there is no remaining user funds or negative yield on the YieldProvider.
   * @param _yieldProvider The yield provider address.
   * @param _vendorExitData Vendor-specific exit data.
   */
  function removeYieldProvider(
    address _yieldProvider,
    bytes memory _vendorExitData
  ) external onlyKnownYieldProvider(_yieldProvider) onlyRole(SET_YIELD_PROVIDER_ROLE) {
    uint256 userFundsCached = _getYieldProviderStorage(_yieldProvider).userFunds;
    if (userFundsCached != 0) {
      revert YieldProviderHasRemainingFunds(userFundsCached);
    }
    _removeYieldProvider(_yieldProvider, _vendorExitData, false, userFundsCached);
  }

  /**
   * @notice Emergency remove a YieldProvider instance from the YieldManager, skipping the regular safety checks.
   * @dev SET_YIELD_PROVIDER_ROLE is required to execute.
   * @dev Without this function, newly reported yield can prevent deregistration of the YieldProvider.
   * @param _yieldProvider The yield provider address.
   * @param _vendorExitData Vendor-specific exit data.
   */
  function emergencyRemoveYieldProvider(
    address _yieldProvider,
    bytes memory _vendorExitData
  ) external onlyKnownYieldProvider(_yieldProvider) onlyRole(SET_YIELD_PROVIDER_ROLE) {
    uint256 userFundsCached = _getYieldProviderStorage(_yieldProvider).userFunds;
    _removeYieldProvider(_yieldProvider, _vendorExitData, true, userFundsCached);
  }

  /**
   * @notice Internal helper function to remove a YieldProvider from the registry.
   * @param _yieldProvider The yield provider address.
   * @param _vendorExitData Vendor-specific exit data passed to the yield provider's exit function.
   * @param _isEmergencyRemove Flag indicating whether this is an emergency removal.
   * @param _userFunds The user funds amount for the yield provider.
   */
  function _removeYieldProvider(
    address _yieldProvider,
    bytes memory _vendorExitData,
    bool _isEmergencyRemove,
    uint256 _userFunds
  ) internal {
    _delegatecallYieldProvider(
      _yieldProvider,
      abi.encodeCall(IYieldProvider.exitVendorContracts, (_yieldProvider, _vendorExitData))
    );

    YieldManagerStorage storage $ = _getYieldManagerStorage();
    if (_userFunds > 0) {
      $.userFundsInYieldProvidersTotal -= _userFunds;
    }
    uint96 yieldProviderIndex = _getYieldProviderStorage(_yieldProvider).yieldProviderIndex;
    address lastYieldProvider = $.yieldProviders[$.yieldProviders.length - 1];
    $.yieldProviderStorage[lastYieldProvider].yieldProviderIndex = yieldProviderIndex;
    $.yieldProviders[yieldProviderIndex] = lastYieldProvider;
    $.yieldProviders.pop();

    delete $.yieldProviderStorage[_yieldProvider];
    emit YieldProviderRemoved(_yieldProvider, _isEmergencyRemove);
  }

  /**
   * @notice Add an address to the allowlist of L2YieldRecipients.
   * @dev SET_L2_YIELD_RECIPIENT_ROLE is required to execute.
   * @param _l2YieldRecipient L2YieldRecipient address.
   */
  function addL2YieldRecipient(address _l2YieldRecipient) external onlyRole(SET_L2_YIELD_RECIPIENT_ROLE) {
    ErrorUtils.revertIfZeroAddress(_l2YieldRecipient);
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    if ($.isL2YieldRecipientKnown[_l2YieldRecipient]) {
      revert L2YieldRecipientAlreadyAdded();
    }
    emit L2YieldRecipientAdded(_l2YieldRecipient);
    $.isL2YieldRecipientKnown[_l2YieldRecipient] = true;
  }

  /**
   * @notice Remove an address from the allow-list of L2YieldRecipients.
   * @dev SET_L2_YIELD_RECIPIENT_ROLE is required to execute.
   * @param _l2YieldRecipient L2YieldRecipient address.
   */
  function removeL2YieldRecipient(
    address _l2YieldRecipient
  ) external onlyKnownL2YieldRecipient(_l2YieldRecipient) onlyRole(SET_L2_YIELD_RECIPIENT_ROLE) {
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    emit L2YieldRecipientRemoved(_l2YieldRecipient);
    $.isL2YieldRecipientKnown[_l2YieldRecipient] = false;
  }

  /**
   * @notice Update withdrawal reserve parameters.
   * @dev WITHDRAWAL_RESERVE_SETTER_ROLE is required to execute.
   * @param _params Data used to update withdrawal reserve parameters.
   */
  function setWithdrawalReserveParameters(
    UpdateReserveParametersConfig memory _params
  ) external onlyRole(WITHDRAWAL_RESERVE_SETTER_ROLE) {
    _setWithdrawalReserveParameters(_params);
  }

  /**
   * @notice Helper function to update withdrawal reserve parameters
   * @dev WITHDRAWAL_RESERVE_SETTER_ROLE is required to execute.
   * @param _params Data used to update withdrawal reserve parameters.
   */
  function _setWithdrawalReserveParameters(UpdateReserveParametersConfig memory _params) internal {
    if (
      _params.minimumWithdrawalReservePercentageBps > MAX_BPS || _params.targetWithdrawalReservePercentageBps > MAX_BPS
    ) {
      revert BpsMoreThan10000();
    }
    if (_params.targetWithdrawalReservePercentageBps <= _params.minimumWithdrawalReservePercentageBps) {
      revert TargetReservePercentageMustBeAboveMinimum();
    }
    if (_params.targetWithdrawalReserveAmount <= _params.minimumWithdrawalReserveAmount) {
      revert TargetReserveAmountMustBeAboveMinimum();
    }
    YieldManagerStorage storage $ = _getYieldManagerStorage();
    emit WithdrawalReserveParametersSet(
      $.minimumWithdrawalReservePercentageBps,
      $.minimumWithdrawalReserveAmount,
      $.targetWithdrawalReservePercentageBps,
      $.targetWithdrawalReserveAmount,
      _params.minimumWithdrawalReservePercentageBps,
      _params.minimumWithdrawalReserveAmount,
      _params.targetWithdrawalReservePercentageBps,
      _params.targetWithdrawalReserveAmount
    );
    $.minimumWithdrawalReservePercentageBps = _params.minimumWithdrawalReservePercentageBps;
    $.minimumWithdrawalReserveAmount = _params.minimumWithdrawalReserveAmount;
    $.targetWithdrawalReservePercentageBps = _params.targetWithdrawalReservePercentageBps;
    $.targetWithdrawalReserveAmount = _params.targetWithdrawalReserveAmount;
  }
}
