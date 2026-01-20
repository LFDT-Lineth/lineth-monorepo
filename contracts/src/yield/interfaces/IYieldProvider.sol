// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { YieldProviderVendor, ProgressOssificationResult, YieldProviderRegistration } from "./YieldTypes.sol";

/**
 * @title Interface for a YieldProvider adaptor contract to handle vendor-specific interactions.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IYieldProvider {
  /// @notice Enumerates operations that can be paused during ossification depending on the yield provider vendor.
  enum OperationType {
    FundYieldProvider,
    ReportYield
  }

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
  /// @notice Thrown when an operation is blocked due to staking pause.
  /// @param operationType The operation that was attempted.
  error OperationNotSupportedDuringStakingPause(OperationType operationType);

  /// @notice Thrown when an operation is blocked because ossification is either pending or complete.
  /// @param operationType The operation that was attempted.
  error OperationNotSupportedDuringOssification(OperationType operationType);

  /// @notice Raised when the YieldManager attempts to add a yield provider with an unexpected vendor identifier.
  error UnknownYieldProviderVendor();

  /// @notice Raised when unpause staking is attempted when ossification is complete.
  error UnpauseStakingForbiddenWhenOssified();

  /// @notice Raised when LST minting is attempted while ossification is either pending or complete.
  error MintLSTDisabledDuringOssification();

  /// @notice Raised when a permissionless unstake request references more than one validator.
  error SingleValidatorOnlyForUnstakePermissionless();

  /// @notice Raised when a permissionless unstake request specifies a zero exit amount.
  error NoValidatorExitForUnstakePermissionless();

  /// @notice Raised when a function is called outside of a `delegatecall` from the YieldManager.
  error ContextIsNotYieldManager();

  /// @notice Raised no vendor exit data is provided.
  error NoVendorExitDataProvided();

  /**
   * @notice Returns the amount of ETH the provider can immediately remit back to the YieldManager.
   * @dev Called via `delegatecall` from the YieldManager.
   * @dev Made a payable function to be `delegatecall-able` from YieldManager.unstakePermissionless().
   * @param _yieldProvider The yield provider address.
   * @return availableBalance The ETH amount that can be withdrawn.
   */
  function withdrawableValue(address _yieldProvider) external payable returns (uint256 availableBalance);

  /**
   * @notice Forwards ETH from the YieldManager to the yield provider.
   * @param _yieldProvider The yield provider address.
   * @param _amount Amount of ETH supplied by the YieldManager.
   */
  function fundYieldProvider(address _yieldProvider, uint256 _amount) external;

  /**
   * @notice Computes and returns earned yield that can be distributed to L2 users.
   * @dev Implementations should apply provider-specific adjustments (obligations, fees, negative
   *      yield) and mutate only `lstLiabilityPrincipal` and `currentNegativeYield` in the
   *      YieldProvider state.
   * @param _yieldProvider The yield provider address.
   * @return newReportedYield New net yield (denominated in ETH) since the prior report.
   * @return outstandingNegativeYield Amount of outstanding negative yield.
   */
  function reportYield(
    address _yieldProvider
  ) external returns (uint256 newReportedYield, uint256 outstandingNegativeYield);

  /**
   * @notice Requests beacon chain withdrawal via EIP-7002 withdrawal contract.
   * @dev Parameters are ABI encoded by the YieldManager and understood by the yield provider.
   * @dev Dynamic withdrawal fee is sourced from `msg.value`
   * @param _yieldProvider The yield provider address.
   * @param _withdrawalParams Provider-specific payload describing the withdrawals to trigger.
   */
  function unstake(address _yieldProvider, bytes memory _withdrawalParams) external payable;

  /**
   * @notice Permissionlessly requests beacon chain withdrawal via EIP-7002 withdrawal contract when reserve is under minimum threshold.
   * @dev Implementations must verify the calldata proof (for example against EIP-4788 beacon roots)
   *      and enforce any provider-specific safety checks. The returned amount is used by the
   *      YieldManager to cap pending withdrawals tracked on L1.
   * @param _yieldProvider The yield provider address.
   * @param _requiredUnstakeAmount Required unstake amount in wei.
   * @param _validatorIndex Validator index for validator to withdraw from.
   * @param _slot Slot of the beacon block for which the proof is generated.
   * @param _withdrawalParams ABI encoded provider parameters.
   * @param _withdrawalParamsProof Proof data (typically a beacon chain Merkle proof).
   * @return unstakedAmountWei Maximum ETH amount expected to be withdrawn as a result of this request (in wei).
   */
  function unstakePermissionless(
    address _yieldProvider,
    uint256 _requiredUnstakeAmount,
    uint64 _validatorIndex,
    uint64 _slot,
    bytes calldata _withdrawalParams,
    bytes calldata _withdrawalParamsProof
  ) external payable returns (uint256 unstakedAmountWei);

  /**
   * @notice Hook called before withdrawing ETH from the YieldProvider.
   * @param _yieldProvider The yield provider address.
   * @param _isPermissionlessReserveDeficitWithdrawal Whether this is a permissionless reserve deficit withdrawal.
   */
  function beforeWithdrawFromYieldProvider(
    address _yieldProvider,
    bool _isPermissionlessReserveDeficitWithdrawal
  ) external;

  /**
   * @notice Withdraws ETH from the provider back into the YieldManager.
   * @param _yieldProvider The yield provider address.
   * @param _amount Amount of ETH to withdraw to the YieldManager.
   */
  function withdrawFromYieldProvider(address _yieldProvider, uint256 _amount) external;

  /**
   * @notice Pauses new beacon chain deposits.
   * @param _yieldProvider The yield provider address.
   */
  function pauseStaking(address _yieldProvider) external;

  /**
   * @notice Resumes beacon chain deposits for the provider after a pause.
   * @param _yieldProvider The yield provider address.
   * @dev Whether to allow staking during ossification is a vendor-specific detail.
   */
  function unpauseStaking(address _yieldProvider) external;

  /**
   * @notice Synchronizes the cached LST liability principal with the latest vendor state.
   * @param _yieldProvider The yield provider address.
   */
  function syncLSTLiabilityPrincipal(address _yieldProvider) external;

  /**
   * @notice Withdraws liquid staking tokens (LST) to a recipient.
   * @dev Implementations must `lstLiabilityPrincipal` state for the yield provider.
   * @param _yieldProvider The yield provider address.
   * @param _amount Amount of LST (denominated in ETH) to withdraw.
   * @param _recipient Address receiving the LST.
   */
  function withdrawLST(address _yieldProvider, uint256 _amount, address _recipient) external;

  /**
   * @notice Begins the provider-specific ossification workflow.
   * @param _yieldProvider The yield provider address.
   */
  function initiateOssification(address _yieldProvider) external;

  /**
   * @notice Process a previously initiated ossification process.
   * @param _yieldProvider The yield provider address.
   * @return progressOssificationResult The operation result.
   */
  function progressPendingOssification(
    address _yieldProvider
  ) external returns (ProgressOssificationResult progressOssificationResult);

  /**
   * @notice Performs vendor-specific initialization logic.
   * @param _vendorInitializationData Vendor-specific initialization data.
   * @return registrationData Data required to register a new YieldProvider with the YieldManager.
   */
  function initializeVendorContracts(
    bytes memory _vendorInitializationData
  ) external returns (YieldProviderRegistration memory registrationData);

  /**
   * @notice Performs vendor-specific exit logic.
   * @param _yieldProvider The yield provider address.
   * @param _vendorExitData Vendor-specific exit data.
   */
  function exitVendorContracts(address _yieldProvider, bytes memory _vendorExitData) external;
}
