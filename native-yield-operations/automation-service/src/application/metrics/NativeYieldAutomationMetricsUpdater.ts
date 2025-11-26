import { IMetricsService } from "@consensys/linea-shared-utils";
import {
  LineaNativeYieldAutomationServiceMetrics,
  OperationTrigger,
} from "../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { RebalanceDirection } from "../../core/entities/RebalanceRequirement.js";
import { Address, Hex } from "viem";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";

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

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.ReportYieldAmountTotal,
      "Total yield amount reported per vault",
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
      LineaNativeYieldAutomationServiceMetrics.LastPeekUnpaidLidoProtocolFees,
      "Unpaid Lido protocol fees from the last peek",
      ["vault_address"],
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
      LineaNativeYieldAutomationServiceMetrics.OperationModeTriggerTotal,
      "Operation mode triggers grouped by mode and triggers",
      ["mode", "trigger"],
    );

    this.metricsService.createCounter(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
      "Operation mode executions grouped by mode",
      ["mode"],
    );

    this.metricsService.createHistogram(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
      OPERATION_MODE_DURATION_BUCKETS,
      "Operation mode execution duration in seconds",
      ["mode"],
    );
  }

  /**
   * Records a rebalance operation amount.
   * Increments the rebalance amount counter for the specified direction.
   *
   * @param {RebalanceDirection.STAKE | RebalanceDirection.UNSTAKE} direction - The direction of the rebalance (STAKE or UNSTAKE).
   * @param {number} amountGwei - The rebalance amount in gwei. Must be greater than 0 to be recorded.
   */
  public recordRebalance(direction: RebalanceDirection.STAKE | RebalanceDirection.UNSTAKE, amountGwei: number): void {
    if (amountGwei <= 0) return;
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
   * Adds to the total yield amount reported for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} amountGwei - The yield amount in gwei. Must be greater than 0 to be recorded.
   */
  public addReportedYieldAmount(vaultAddress: Address, amountGwei: number): void {
    if (amountGwei <= 0) return;
    this.metricsService.incrementCounter(
      LineaNativeYieldAutomationServiceMetrics.ReportYieldAmountTotal,
      { vault_address: vaultAddress },
      amountGwei,
    );
  }

  /**
   * Sets the outstanding negative yield from the last peeked yield report for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} negativeYield - The negative yield amount. Must be non-negative to be recorded.
   * @returns {Promise<void>} A promise that resolves when the gauge is set.
   */
  public async setLastPeekedNegativeYieldReport(vaultAddress: Address, negativeYield: number): Promise<void> {
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
   * @returns {Promise<void>} A promise that resolves when the gauge is set.
   */
  public async setLastPeekedPositiveYieldReport(vaultAddress: Address, yieldAmount: number): Promise<void> {
    if (yieldAmount < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastPeekedPositiveYieldReport,
      { vault_address: vaultAddress },
      yieldAmount,
    );
  }

  /**
   * Sets the unpaid Lido protocol fees from the last peek for a specific vault.
   *
   * @param {Address} vaultAddress - The address of the vault.
   * @param {number} feesAmount - The unpaid fees amount. Must be non-negative to be recorded.
   * @returns {Promise<void>} A promise that resolves when the gauge is set.
   */
  public async setLastPeekUnpaidLidoProtocolFees(vaultAddress: Address, feesAmount: number): Promise<void> {
    if (feesAmount < 0) return;
    this.metricsService.setGauge(
      LineaNativeYieldAutomationServiceMetrics.LastPeekUnpaidLidoProtocolFees,
      { vault_address: vaultAddress },
      feesAmount,
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
   * Increments the counter for operation mode triggers, grouped by mode and trigger type.
   *
   * @param {OperationMode} mode - The operation mode that was triggered.
   * @param {OperationTrigger} trigger - The trigger that caused the mode to be activated.
   */
  public incrementOperationModeTrigger(mode: OperationMode, trigger: OperationTrigger): void {
    this.metricsService.incrementCounter(LineaNativeYieldAutomationServiceMetrics.OperationModeTriggerTotal, {
      mode,
      trigger,
    });
  }

  /**
   * Increments the counter for operation mode executions, grouped by mode.
   *
   * @param {OperationMode} mode - The operation mode that was executed.
   */
  public incrementOperationModeExecution(mode: OperationMode): void {
    this.metricsService.incrementCounter(LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal, {
      mode,
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
