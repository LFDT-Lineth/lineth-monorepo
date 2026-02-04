import { IMetricsService } from "@consensys/linea-shared-utils";
import {
  LineaNativeYieldAutomationServiceMetrics,
  OperationModeExecutionStatus,
} from "../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { RebalanceDirection } from "../../core/entities/RebalanceRequirement.js";
import { Address, Hex } from "viem";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";

/**
 * Maps RebalanceDirection enum to staking direction label string for metrics.
 *
 * @param {RebalanceDirection} direction - The rebalance direction.
 * @returns {string} The staking direction label value ("STAKING", "UNSTAKING", or "NONE").
 */
function mapRebalanceDirectionToStakingDirection(direction: RebalanceDirection): string {
  switch (direction) {
    case RebalanceDirection.STAKE:
      return "STAKING";
    case RebalanceDirection.UNSTAKE:
      return "UNSTAKING";
    case RebalanceDirection.NONE:
      return "NONE";
    default:
      return "NONE";
  }
}

// Buckets range up to 20 minutes to account for long-running modes.
const OPERATION_MODE_DURATION_BUCKETS = [1, 5, 10, 30, 60, 120, 180, 300, 600, 900, 1200];

/**
 * Focused on defining the specific metrics, and methods for updating them.
 * Handles creation and updates of all metrics for the Native Yield Automation Service,
 * including rebalances, validator operations, vault reporting, fees, and operation mode tracking.
 */
export class NativeYieldAutomationMetricsUpdater implements INativeYieldAutomationMetricsUpdater {
  /**
   * Creates a new NativeYieldAutomationMetricsUpdater instance.
   * Initializes all metrics (counters, gauges, and histograms) used by the service.
   *
   * @param {IMetricsService<LineaNativeYieldAutomationServiceMetrics>} metricsService - The metrics service used to create and update metrics.
   */
  constructor(private readonly metricsService: IMetricsService<LineaNativeYieldAutomationServiceMetrics>) {
    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.RebalanceAmountTotal,
      "Total rebalance amount between L1MessageService and YieldProvider",
      ["direction", "type"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.ValidatorPartialUnstakeAmountTotal,
      "Total amount partially unstaked per validator",
      ["validator_pubkey"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.ValidatorExitTotal,
      "Total validator exits initiated by automation",
      ["validator_pubkey"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.ValidatorStakedAmountGwei,
      "Amount staked in a validator in gwei",
      ["pubkey"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.LidoVaultAccountingReportSubmittedTotal,
      "Accounting reports submitted to Lido per vault",
      ["vault_address"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal,
      "Yield reports submitted to YieldManager per vault",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastPeekedNegativeYieldReport,
      "Outstanding negative yield from the last peeked yield report",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastPeekedPositiveYieldReport,
      "Positive yield amount from the last peeked yield report",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastSettleableLidoFees,
      "Settleable Lido protocol fees from the last query",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastVaultReportTimestamp,
      "Timestamp from the latest vault report",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.YieldReportedCumulative,
      "Cumulative yield reported from the YieldManager contract",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LstLiabilityPrincipalGwei,
      "LST liability principal from the YieldManager contract",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastReportedNegativeYield,
      "Last reported negative yield from the YieldManager contract",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LidoLstLiabilityGwei,
      "Lido LST liability in gwei from Lido accounting reports",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingPartialWithdrawalsGwei,
      "Total pending partial withdrawals in gwei",
      [],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalValidatorBalanceGwei,
      "Total validator balance in gwei",
      [],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingDepositGwei,
      "Total pending deposits in gwei",
      [],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingPartialWithdrawalQueueAmountGwei,
      "Pending partial withdrawal queue amount in gwei",
      ["pubkey", "withdrawable_epoch"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingDepositQueueAmountGwei,
      "Pending deposit queue amount in gwei",
      ["pubkey", "slot"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
      "Pending exit queue amount in gwei",
      ["pubkey", "exit_epoch", "slashed"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingExitGwei,
      "Total pending exit amount in gwei",
      [],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
      "Pending full withdrawal queue amount in gwei",
      ["pubkey", "withdrawable_epoch", "slashed"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingFullWithdrawalGwei,
      "Total pending full withdrawal amount in gwei",
      [],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.NodeOperatorFeesPaidTotal,
      "Node operator fees paid by automation per vault",
      ["vault_address"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.LiabilitiesPaidTotal,
      "Liabilities paid by automation per vault",
      ["vault_address"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.LidoFeesPaidTotal,
      "Lido fees paid by automation per vault",
      ["vault_address"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
      "Operation mode executions grouped by mode and status",
      ["mode", "status"],
    );

    this.metricsService.createHistogram(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
      OPERATION_MODE_DURATION_BUCKETS,
      "Operation mode execution duration in seconds",
      ["mode"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.StakingDepositQuotaExceeded,
      "Total number of times the staking deposit quota has been exceeded",
      ["vault_address"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.ActualRebalanceRequirementGwei,
      "Original rebalance requirement (in gwei) before applying tolerance band, circuit breaker, or rate limit",
      ["vault_address", "staking_direction"],
    );

    this.metricsService.createGauge(
      LineaNativeYieldAutomationServiceMetrics.ReportedRebalanceRequirementGwei,
      "Reported rebalance requirement (in gwei) after applying tolerance band, circuit breaker, and rate limit",
      ["vault_address", "staking_direction"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.ContractEstimateGasError,
      "Total number of contract estimateGas errors",
      ["contract_address", "rawRevertData", "errorName"],
    );
  }

  /**
   * Records a rebalance operation amount.
   * Increments the rebalance amount counter for the specified direction.
   *
   * @param {RebalanceDirection} direction - The direction of the rebalance (NONE, STAKE, or UNSTAKE).
   * @param {number} amountGwei - The rebalance amount in gwei.
   */
  public recordRebalance(direction: RebalanceDirection, amountGwei: number): void {
    if (amountGwei < 0) return;
    this.metricsService.incrementCounter(
      LineaNativeYieldAutomationServiceMetrics.RebalanceAmountTotal,
      { direction },
      amountGwei,
    );
  }

  /**
   * Adds to the total amount partially unstaked for a specific validator.
   *
   * @param {Hex} validatorPubkey - The validator's public key in hex format.
   * @param {number} amountGwei - The partial unstake amount in gwei. Must be greater than 0 to be recorded.
   */
  public addValidatorPartialUnstakeAmount(validatorPubkey: Hex, amountGwei: number): void {
    if (amountGwei <= 0) return;
    this.metricsService.incrementCounter(
      LineaNativeYieldAutomationServiceMetrics.ValidatorPartialUnstakeAmountTotal,
      { validator_pubkey: validatorPubkey },
      amountGwei,
    );
  }

  /**
   * Increments the counter for validator exits initiated by automation.
   *
   * @param {Hex} validatorPubkey - The validator's public key in hex format.
   * @param {number} [count=1] - The number of exits to record. Must be greater than 0 to be recorded.
   */
  public incrementValidatorExit(validatorPubkey: Hex, count: number = 1): void {
    if (count <= 0) return;
    this.metricsService.incrementCounter(
      LineaNativeYieldAutomationServiceMetrics.ValidatorExitTotal,
      { validator_pubkey: validatorPubkey },
      count,
    );
  }

  /**
   * Sets the amount staked in a validator in gwei.
   *
   * @param {Hex} pubkey - The validator's public key in hex format.
   * @param {number} amountGwei - The staked amount in gwei. Must be non-negative to be recorded.
   */
  public setValidatorStakedAmountGwei(pubkey: Hex, amountGwei: number): void {
    if (amountGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.ValidatorStakedAmountGwei,
      { pubkey },
      amountGwei,
    );
  }

  /**
   * Increments the counter for accounting reports submitted to Lido for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   */
  public incrementLidoVaultAccountingReport(vaultAddress: Address): void {
    this.metricsService.incrementCounter(
      LineaNativeYieldAutomationServiceMetrics.LidoVaultAccountingReportSubmittedTotal,
      { vault_address: vaultAddress },
    );
  }

  /**
   * Increments the counter for yield reports submitted to YieldManager for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   */
  public incrementReportYield(vaultAddress: Address): void {
    this.metricsService.incrementCounter(LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal, {
      vault_address: vaultAddress,
    });
  }

  /**
   * Sets the outstanding negative yield from the last peeked yield report for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} negativeYield - The negative yield amount. Must be non-negative to be recorded.
   */
  public setLastPeekedNegativeYieldReport(vaultAddress: Address, negativeYield: number): void {
    if (negativeYield < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastPeekedNegativeYieldReport,
      { vault_address: vaultAddress },
      negativeYield,
    );
  }

  /**
   * Sets the positive yield amount from the last peeked yield report for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} yieldAmount - The yield amount. Must be non-negative to be recorded.
   */
  public setLastPeekedPositiveYieldReport(vaultAddress: Address, yieldAmount: number): void {
    if (yieldAmount < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastPeekedPositiveYieldReport,
      { vault_address: vaultAddress },
      yieldAmount,
    );
  }

  /**
   * Sets the settleable Lido protocol fees from the last query for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} feesAmount - The settleable fees amount. Must be non-negative to be recorded.
   */
  public setLastSettleableLidoFees(vaultAddress: Address, feesAmount: number): void {
    if (feesAmount < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastSettleableLidoFees,
      { vault_address: vaultAddress },
      feesAmount,
    );
  }

  /**
   * Sets the timestamp from the latest vault report for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} timestamp - The timestamp in seconds (Unix timestamp). Must be non-negative to be recorded.
   */
  public setLastVaultReportTimestamp(vaultAddress: Address, timestamp: number): void {
    if (timestamp < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastVaultReportTimestamp,
      { vault_address: vaultAddress },
      timestamp,
    );
  }

  /**
   * Sets the cumulative yield reported from the YieldManager contract for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The cumulative yield amount in gwei. Must be non-negative to be recorded.
   */
  public setYieldReportedCumulative(vaultAddress: Address, amountGwei: number): void {
    if (amountGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.YieldReportedCumulative,
      { vault_address: vaultAddress },
      amountGwei,
    );
  }

  /**
   * Sets the LST liability principal from the YieldManager contract for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The LST liability principal amount in gwei. Must be non-negative to be recorded.
   */
  public setLstLiabilityPrincipalGwei(vaultAddress: Address, amountGwei: number): void {
    if (amountGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LstLiabilityPrincipalGwei,
      { vault_address: vaultAddress },
      amountGwei,
    );
  }

  /**
   * Sets the last reported negative yield from the YieldManager contract for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The last reported negative yield amount in gwei. Must be non-negative to be recorded.
   */
  public setLastReportedNegativeYield(vaultAddress: Address, amountGwei: number): void {
    if (amountGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastReportedNegativeYield,
      { vault_address: vaultAddress },
      amountGwei,
    );
  }

  /**
   * Sets the Lido LST liability in gwei from Lido accounting reports for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The Lido LST liability amount in gwei. Must be non-negative to be recorded.
   */
  public setLidoLstLiabilityGwei(vaultAddress: Address, amountGwei: number): void {
    if (amountGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LidoLstLiabilityGwei,
      { vault_address: vaultAddress },
      amountGwei,
    );
  }

  /**
   * Sets the total pending partial withdrawals in gwei from the last query.
   *
   * @param {number} totalPendingPartialWithdrawalsGwei - The total pending partial withdrawals amount in gwei. Must be non-negative to be recorded.
   */
  public setLastTotalPendingPartialWithdrawalsGwei(totalPendingPartialWithdrawalsGwei: number): void {
    if (totalPendingPartialWithdrawalsGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingPartialWithdrawalsGwei,
      {},
      totalPendingPartialWithdrawalsGwei,
    );
  }

  /**
   * Sets the total validator balance in gwei from the last query.
   *
   * @param {number} totalValidatorBalanceGwei - The total validator balance amount in gwei. Must be non-negative to be recorded.
   */
  public setLastTotalValidatorBalanceGwei(totalValidatorBalanceGwei: number): void {
    if (totalValidatorBalanceGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalValidatorBalanceGwei,
      {},
      totalValidatorBalanceGwei,
    );
  }

  /**
   * Sets the total pending deposits in gwei from the last query.
   *
   * @param {number} totalPendingDepositGwei - The total pending deposits amount in gwei. Must be non-negative to be recorded.
   */
  public setLastTotalPendingDepositGwei(totalPendingDepositGwei: number): void {
    if (totalPendingDepositGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingDepositGwei,
      {},
      totalPendingDepositGwei,
    );
  }

  /**
   * Sets the pending partial withdrawal queue amount in gwei for a specific validator and withdrawable epoch.
   *
   * @param {Hex} pubkey - The validator's public key in hex format.
   * @param {number} withdrawableEpoch - The epoch when the withdrawal becomes available. Must be non-negative.
   * @param {number} amountGwei - The withdrawal amount in gwei. Must be non-negative to be recorded.
   */
  public setPendingPartialWithdrawalQueueAmountGwei(pubkey: Hex, withdrawableEpoch: number, amountGwei: number): void {
    if (amountGwei < 0 || withdrawableEpoch < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingPartialWithdrawalQueueAmountGwei,
      { pubkey, withdrawable_epoch: withdrawableEpoch.toString() },
      amountGwei,
    );
  }

  /**
   * Sets the pending deposit queue amount in gwei for a specific validator and slot.
   *
   * @param {Hex} pubkey - The validator's public key in hex format.
   * @param {number} slot - The slot number. Must be non-negative.
   * @param {number} amountGwei - The deposit amount in gwei. Must be non-negative to be recorded.
   */
  public setPendingDepositQueueAmountGwei(pubkey: Hex, slot: number, amountGwei: number): void {
    if (amountGwei < 0 || slot < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingDepositQueueAmountGwei,
      { pubkey, slot: slot.toString() },
      amountGwei,
    );
  }

  /**
   * Sets the pending exit queue amount in gwei for a specific validator and exit epoch.
   *
   * @param {Hex} pubkey - The validator's public key in hex format.
   * @param {number} exitEpoch - The epoch when the exit becomes available. Must be non-negative.
   * @param {number} amountGwei - The exit amount in gwei. Must be non-negative to be recorded.
   * @param {boolean} slashed - Whether the validator has been slashed.
   */
  public setPendingExitQueueAmountGwei(pubkey: Hex, exitEpoch: number, amountGwei: number, slashed: boolean): void {
    if (amountGwei < 0 || exitEpoch < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
      { pubkey, exit_epoch: exitEpoch.toString(), slashed: slashed.toString() },
      amountGwei,
    );
  }

  /**
   * Sets the total pending exit amount in gwei from the last query.
   *
   * @param {number} totalPendingExitGwei - The total pending exit amount in gwei. Must be non-negative to be recorded.
   */
  public setLastTotalPendingExitGwei(totalPendingExitGwei: number): void {
    if (totalPendingExitGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingExitGwei,
      {},
      totalPendingExitGwei,
    );
  }

  /**
   * Sets the pending full withdrawal queue amount in gwei for a specific validator and withdrawable epoch.
   *
   * @param {Hex} pubkey - The validator's public key in hex format.
   * @param {number} withdrawableEpoch - The epoch when the withdrawal becomes available. Must be non-negative.
   * @param {number} amountGwei - The withdrawal amount in gwei. Must be non-negative to be recorded.
   * @param {boolean} slashed - Whether the validator has been slashed.
   */
  public setPendingFullWithdrawalQueueAmountGwei(
    pubkey: Hex,
    withdrawableEpoch: number,
    amountGwei: number,
    slashed: boolean,
  ): void {
    if (amountGwei < 0 || withdrawableEpoch < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
      { pubkey, withdrawable_epoch: withdrawableEpoch.toString(), slashed: slashed.toString() },
      amountGwei,
    );
  }

  /**
   * Sets the total pending full withdrawal amount in gwei from the last query.
   *
   * @param {number} totalPendingFullWithdrawalGwei - The total pending full withdrawal amount in gwei. Must be non-negative to be recorded.
   */
  public setLastTotalPendingFullWithdrawalGwei(totalPendingFullWithdrawalGwei: number): void {
    if (totalPendingFullWithdrawalGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastTotalPendingFullWithdrawalGwei,
      {},
      totalPendingFullWithdrawalGwei,
    );
  }

  /**
   * Adds to the total node operator fees paid by automation for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The fees amount in gwei. Must be greater than 0 to be recorded.
   */
  public addNodeOperatorFeesPaid(vaultAddress: Address, amountGwei: number): void {
    this._incrementVaultAmountCounter(
      LineaNativeYieldAutomationServiceMetrics.NodeOperatorFeesPaidTotal,
      vaultAddress,
      amountGwei,
    );
  }

  /**
   * Adds to the total liabilities paid by automation for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The liabilities amount in gwei. Must be greater than 0 to be recorded.
   */
  public addLiabilitiesPaid(vaultAddress: Address, amountGwei: number): void {
    this._incrementVaultAmountCounter(
      LineaNativeYieldAutomationServiceMetrics.LiabilitiesPaidTotal,
      vaultAddress,
      amountGwei,
    );
  }

  /**
   * Adds to the total Lido fees paid by automation for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The fees amount in gwei. Must be greater than 0 to be recorded.
   */
  public addLidoFeesPaid(vaultAddress: Address, amountGwei: number): void {
    this._incrementVaultAmountCounter(
      LineaNativeYieldAutomationServiceMetrics.LidoFeesPaidTotal,
      vaultAddress,
      amountGwei,
    );
  }

  /**
   * Increments the counter for operation mode executions, grouped by mode and status.
   *
   * @param {OperationMode} mode - The operation mode that was executed.
   * @param {OperationModeExecutionStatus} [status=OperationModeExecutionStatus.Success] - The execution status. Defaults to Success.
   */
  public incrementOperationModeExecution(
    mode: OperationMode,
    status: OperationModeExecutionStatus = OperationModeExecutionStatus.Success,
  ): void {
    this.metricsService.incrementCounter(LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal, {
      mode,
      status,
    });
  }

  /**
   * Records the execution duration of an operation mode in a histogram.
   * Uses buckets that range up to 20 minutes to account for long-running modes.
   *
   * @param {OperationMode} mode - The operation mode that was executed.
   * @param {number} durationSeconds - The duration in seconds. Must be non-negative to be recorded.
   */
  public recordOperationModeDuration(mode: OperationMode, durationSeconds: number): void {
    if (durationSeconds < 0) return;
    this.metricsService.addValueToHistogram(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
      durationSeconds,
      { mode },
    );
  }

  /**
   * Increments the counter for staking deposit quota exceeded events.
   *
   * @param {Address} vaultAddress - The address of the vault.
   */
  public incrementStakingDepositQuotaExceeded(vaultAddress: Address): void {
    this.metricsService.incrementCounter(LineaNativeYieldAutomationServiceMetrics.StakingDepositQuotaExceeded, {
      vault_address: vaultAddress,
    });
  }

  /**
   * Sets the original rebalance requirement (in gwei) before applying tolerance band, circuit breaker, or rate limit.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} requirementGwei - The requirement amount in gwei. Must be non-negative to be recorded.
   * @param {RebalanceDirection} direction - The rebalance direction.
   */
  public setActualRebalanceRequirement(
    vaultAddress: Address,
    requirementGwei: number,
    direction: RebalanceDirection,
  ): void {
    if (requirementGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.ActualRebalanceRequirementGwei,
      { vault_address: vaultAddress, staking_direction: mapRebalanceDirectionToStakingDirection(direction) },
      requirementGwei,
    );
  }

  /**
   * Sets the reported rebalance requirement (in gwei) after applying tolerance band, circuit breaker, and rate limit.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} requirementGwei - The requirement amount in gwei. Must be non-negative to be recorded.
   * @param {RebalanceDirection} direction - The rebalance direction.
   */
  public setReportedRebalanceRequirement(
    vaultAddress: Address,
    requirementGwei: number,
    direction: RebalanceDirection,
  ): void {
    if (requirementGwei < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.ReportedRebalanceRequirementGwei,
      { vault_address: vaultAddress, staking_direction: mapRebalanceDirectionToStakingDirection(direction) },
      requirementGwei,
    );
  }

  /**
   * Increments the counter for contract estimateGas errors.
   *
   * @param {Address} contractAddress - The contract address where the error occurred.
   * @param {string} rawRevertData - The raw revert data (hex string).
   * @param {string} [errorName] - The decoded error name (if available, otherwise "unknown").
   */
  public incrementContractEstimateGasError(contractAddress: Address, rawRevertData: string, errorName?: string): void {
    this.metricsService.incrementCounter(LineaNativeYieldAutomationServiceMetrics.ContractEstimateGasError, {
      contract_address: contractAddress,
      rawRevertData: rawRevertData,
      errorName: errorName ?? "unknown",
    });
  }

  /**
   * Internal helper method to increment a vault-specific amount counter.
   *
   * @param {LineaNativeYieldAutomationServiceMetrics} metric - The metric to increment.
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The amount in gwei. Must be greater than 0 to be recorded.
   */
  private _incrementVaultAmountCounter(
    metric: LineaNativeYieldAutomationServiceMetrics,
    vaultAddress: Address,
    amountGwei: number,
  ): void {
    if (amountGwei <= 0) return;
    this.metricsService.incrementCounter(metric, { vault_address: vaultAddress }, amountGwei);
  }
}
