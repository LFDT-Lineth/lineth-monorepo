// Take operation results and record the relevant figures into metrics

import { Address, TransactionReceipt } from "viem";
import { Result } from "neverthrow";
import { IOperationModeMetricsRecorder } from "../../core/metrics/IOperationModeMetricsRecorder.js";
import { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import { ILogger, weiToGweiNumber } from "@consensys/linea-shared-utils";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { IVaultHub } from "../../core/clients/contracts/IVaultHub.js";
import { DashboardContractClient } from "../../clients/contracts/DashboardContractClient.js";
import { RebalanceDirection } from "../../core/entities/RebalanceRequirement.js";

/**
 * Take operation results and record the relevant figures into metrics.
 * Extracts transaction data from operation results and updates metrics for various operation modes,
 * including progress ossification, yield reporting, safe withdrawals, and fund transfers.
 */
export class OperationModeMetricsRecorder implements IOperationModeMetricsRecorder {
  /**
   * Creates a new OperationModeMetricsRecorder instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating metrics.
   * @param {IYieldManager<TransactionReceipt>} yieldManagerClient - Client for interacting with YieldManager contracts.
   * @param {IVaultHub<TransactionReceipt>} vaultHubClient - Client for interacting with VaultHub contracts.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly yieldManagerClient: IYieldManager<TransactionReceipt>,
    private readonly vaultHubClient: IVaultHub<TransactionReceipt>,
  ) {}

  /**
   * Records metrics for progress ossification operations.
   * Extracts node operator fees, Lido fees, and liability payments from the transaction receipt
   * and updates the corresponding metrics for the vault.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {Result<TransactionReceipt | undefined, Error>} txReceiptResult - The transaction receipt result (may be an error or undefined).
   * @returns {Promise<void>} A promise that resolves when metrics are recorded (or silently returns if receipt is missing or error).
   */
  async recordProgressOssificationMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void> {
    if (txReceiptResult.isErr()) {
      this.logger.warn(
        "recordProgressOssificationMetrics - transaction receipt result is error, skipping metrics recording",
        {
          error: txReceiptResult.error,
        },
      );
      return;
    }
    const receipt = txReceiptResult.value;
    if (!receipt) {
      this.logger.warn("recordProgressOssificationMetrics - receipt is undefined, skipping metrics recording");
      return;
    }

    const [vault, dashboard] = await Promise.all([
      this.yieldManagerClient.getLidoStakingVaultAddress(yieldProvider),
      this.yieldManagerClient.getLidoDashboardAddress(yieldProvider),
    ]);

    const dashboardClient = DashboardContractClient.getOrCreate(dashboard);
    const nodeOperatorFeesDisbursed = dashboardClient.getNodeOperatorFeesPaidFromTxReceipt(receipt);
    this.metricsUpdater.addNodeOperatorFeesPaid(vault, weiToGweiNumber(nodeOperatorFeesDisbursed));

    const lidoFeePayment = this.vaultHubClient.getLidoFeePaymentFromTxReceipt(receipt);
    this.metricsUpdater.addLidoFeesPaid(vault, weiToGweiNumber(lidoFeePayment));

    const liabilityPayment = this.vaultHubClient.getLiabilityPaymentFromTxReceipt(receipt);
    this.metricsUpdater.addLiabilitiesPaid(vault, weiToGweiNumber(liabilityPayment));
  }

  /**
   * Records metrics for yield reporting operations.
   * Extracts yield report data, fees, and liability payments from the transaction receipt
   * and updates metrics including reported yield amount, negative yield, and fee payments.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {Result<TransactionReceipt | undefined, Error>} txReceiptResult - The transaction receipt result (may be an error or undefined).
   * @returns {Promise<void>} A promise that resolves when metrics are recorded (or silently returns if receipt is missing, error, or yield report not found).
   */
  async recordReportYieldMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void> {
    if (txReceiptResult.isErr()) {
      this.logger.warn("recordReportYieldMetrics - transaction receipt result is error, skipping metrics recording", {
        error: txReceiptResult.error,
      });
      return;
    }
    const receipt = txReceiptResult.value;
    if (!receipt) {
      this.logger.warn("recordReportYieldMetrics - receipt is undefined, skipping metrics recording");
      return;
    }

    const yieldReport = this.yieldManagerClient.getYieldReportFromTxReceipt(receipt);
    if (yieldReport === undefined) {
      this.logger.warn("recordReportYieldMetrics - yield report not found in receipt, skipping metrics recording");
      return;
    }

    const [vault, dashboard] = await Promise.all([
      this.yieldManagerClient.getLidoStakingVaultAddress(yieldReport.yieldProvider),
      this.yieldManagerClient.getLidoDashboardAddress(yieldReport.yieldProvider),
    ]);

    this.metricsUpdater.incrementReportYield(vault);

    const dashboardClient = DashboardContractClient.getOrCreate(dashboard);
    const nodeOperatorFeesDisbursed = dashboardClient.getNodeOperatorFeesPaidFromTxReceipt(receipt);
    this.metricsUpdater.addNodeOperatorFeesPaid(vault, weiToGweiNumber(nodeOperatorFeesDisbursed));

    const lidoFeePayment = this.vaultHubClient.getLidoFeePaymentFromTxReceipt(receipt);
    this.metricsUpdater.addLidoFeesPaid(vault, weiToGweiNumber(lidoFeePayment));

    const liabilityPayment = this.vaultHubClient.getLiabilityPaymentFromTxReceipt(receipt);
    this.metricsUpdater.addLiabilitiesPaid(vault, weiToGweiNumber(liabilityPayment));
  }

  /**
   * Records metrics for safe withdrawal operations.
   * Extracts withdrawal event data and liability payments from the transaction receipt
   * and updates rebalance metrics (UNSTAKE direction) and liability payment metrics.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {Result<TransactionReceipt | undefined, Error>} txReceiptResult - The transaction receipt result (may be an error or undefined).
   * @returns {Promise<void>} A promise that resolves when metrics are recorded (or silently returns if receipt is missing, error, or withdrawal event not found).
   */
  async recordSafeWithdrawalMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void> {
    if (txReceiptResult.isErr()) {
      this.logger.warn(
        "recordSafeWithdrawalMetrics - transaction receipt result is error, skipping metrics recording",
        {
          error: txReceiptResult.error,
        },
      );
      return;
    }
    const receipt = txReceiptResult.value;
    if (!receipt) {
      this.logger.warn("recordSafeWithdrawalMetrics - receipt is undefined, skipping metrics recording");
      return;
    }

    const event = this.yieldManagerClient.getWithdrawalEventFromTxReceipt(receipt);
    if (!event) {
      this.logger.warn(
        "recordSafeWithdrawalMetrics - withdrawal event not found in receipt, skipping metrics recording",
      );
      return;
    }
    const { reserveIncrementAmount } = event;

    this.metricsUpdater.recordRebalance(RebalanceDirection.UNSTAKE, weiToGweiNumber(reserveIncrementAmount));

    const vault = await this.yieldManagerClient.getLidoStakingVaultAddress(yieldProvider);
    const liabilityPayment = this.vaultHubClient.getLiabilityPaymentFromTxReceipt(receipt);
    this.metricsUpdater.addLiabilitiesPaid(vault, weiToGweiNumber(liabilityPayment));
  }

  /**
   * Records metrics for fund transfer operations.
   * Extracts liability payments from the transaction receipt and updates the corresponding metrics.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {Result<TransactionReceipt | undefined, Error>} txReceiptResult - The transaction receipt result (may be an error or undefined).
   * @returns {Promise<void>} A promise that resolves when metrics are recorded (or silently returns if receipt is missing or error).
   */
  async recordTransferFundsMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void> {
    if (txReceiptResult.isErr()) {
      this.logger.warn("recordTransferFundsMetrics - transaction receipt result is error, skipping metrics recording", {
        error: txReceiptResult.error,
      });
      return;
    }
    const receipt = txReceiptResult.value;
    if (!receipt) {
      this.logger.warn("recordTransferFundsMetrics - receipt is undefined, skipping metrics recording");
      return;
    }

    const vault = await this.yieldManagerClient.getLidoStakingVaultAddress(yieldProvider);
    const liabilityPayment = this.vaultHubClient.getLiabilityPaymentFromTxReceipt(receipt);
    this.metricsUpdater.addLiabilitiesPaid(vault, weiToGweiNumber(liabilityPayment));
  }
}
