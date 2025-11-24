import { Address, TransactionReceipt } from "viem";
import { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import { IOperationModeProcessor } from "../../core/services/operation-mode/IOperationModeProcessor.js";
import { ILogger, attempt, msToSeconds } from "@consensys/linea-shared-utils";
import { ILazyOracle } from "../../core/clients/contracts/ILazyOracle.js";
import { ILidoAccountingReportClient } from "../../core/clients/ILidoAccountingReportClient.js";
import { IBeaconChainStakingClient } from "../../core/clients/IBeaconChainStakingClient.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";
import { IOperationModeMetricsRecorder } from "../../core/metrics/IOperationModeMetricsRecorder.js";

/**
 * Processor for OSSIFICATION_PENDING_MODE operations.
 * Handles ossification pending state by performing max unstake, submitting vault reports,
 * progressing pending ossification, and performing max withdrawals if ossification completes.
 */
export class OssificationPendingProcessor implements IOperationModeProcessor {
  /**
   * Creates a new OssificationPendingProcessor instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating operation mode metrics.
   * @param {IOperationModeMetricsRecorder} operationModeMetricsRecorder - Service for recording operation mode metrics from transaction receipts.
   * @param {IYieldManager<TransactionReceipt>} yieldManagerContractClient - Client for interacting with YieldManager contracts.
   * @param {ILazyOracle<TransactionReceipt>} lazyOracleContractClient - Client for waiting on LazyOracle events.
   * @param {ILidoAccountingReportClient} lidoAccountingReportClient - Client for submitting Lido accounting reports.
   * @param {IBeaconChainStakingClient} beaconChainStakingClient - Client for managing beacon chain staking operations.
   * @param {Address} yieldProvider - The yield provider address to process.
   * @param {boolean} shouldSubmitVaultReport - Whether to submit the vault accounting report. Can be set to false if other actors are expected to submit.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly operationModeMetricsRecorder: IOperationModeMetricsRecorder,
    private readonly yieldManagerContractClient: IYieldManager<TransactionReceipt>,
    private readonly lazyOracleContractClient: ILazyOracle<TransactionReceipt>,
    private readonly lidoAccountingReportClient: ILidoAccountingReportClient,
    private readonly beaconChainStakingClient: IBeaconChainStakingClient,
    private readonly yieldProvider: Address,
    private readonly shouldSubmitVaultReport: boolean,
  ) {}

  /**
   * Executes one processing cycle:
   * - Waits for the next `VaultsReportDataUpdated` event **or** a timeout, whichever happens first.
   * - Once triggered, runs the main processing logic (`_process()`).
   * - Always cleans up the event watcher afterward.
   * Records operation mode trigger metrics and execution duration metrics.
   *
   * @returns {Promise<void>} A promise that resolves when the processing cycle completes.
   */
  public async process(): Promise<void> {
    const triggerEvent = await this.lazyOracleContractClient.waitForVaultsReportDataUpdatedEvent();
    this.metricsUpdater.incrementOperationModeTrigger(OperationMode.OSSIFICATION_PENDING_MODE, triggerEvent.result);
    const startedAt = performance.now();
    await this._process();
    const durationMs = performance.now() - startedAt;
    this.metricsUpdater.recordOperationModeDuration(OperationMode.OSSIFICATION_PENDING_MODE, msToSeconds(durationMs));
  }

  /**
   * Main processing loop:
   * 1. Max unstake - Submit maximum available withdrawal requests from beacon chain
   * 2. Submit vault report - Fetch and submit latest vault report
   * 3. Process Pending Ossification - Progress pending ossification (stops if failed)
   * 4. Max withdraw if ossified - Perform max safe withdrawal if ossification completed
   *
   * @returns {Promise<void>} A promise that resolves when processing completes (or early returns if ossification fails).
   */
  private async _process(): Promise<void> {
    // Max unstake
    this.logger.info("_process - performing max unstake from beacon chain");
    await attempt(
      this.logger,
      () => this.beaconChainStakingClient.submitMaxAvailableWithdrawalRequests(),
      "submitMaxAvailableWithdrawalRequests failed (tolerated)",
    );

    // Submit vault report if available and enabled
    if (this.shouldSubmitVaultReport) {
      this.logger.info("_process - Fetching latest vault report");
      const vault = await this.yieldManagerContractClient.getLidoStakingVaultAddress(this.yieldProvider);
      await this.lidoAccountingReportClient.getLatestSubmitVaultReportParams(vault);
      this.logger.info("_process - Submitting latest vault report");
      const vaultReportResult = await attempt(
        this.logger,
        () => this.lidoAccountingReportClient.submitLatestVaultReport(vault),
        "submitLatestVaultReport failed (tolerated)",
      );
      if (vaultReportResult.isOk()) {
        this.logger.info("_process - Successfully submitted latest vault report");
        this.metricsUpdater.incrementLidoVaultAccountingReport(vault);
      }
    } else {
      this.logger.info("_process - Skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)");
    }

    // Process Pending Ossification

    const ossificationResult = await attempt(
      this.logger,
      () => this.yieldManagerContractClient.progressPendingOssification(this.yieldProvider),
      "_process - progressPendingOssification failed",
    );
    // Stop if failed.
    if (ossificationResult.isErr()) return;

    await this.operationModeMetricsRecorder.recordProgressOssificationMetrics(this.yieldProvider, ossificationResult);
    this.logger.info("_process - Ossification completed, performing max safe withdrawal");

    // Max withdraw if ossified
    if (await this.yieldManagerContractClient.isOssified(this.yieldProvider)) {
      const withdrawalResult = await attempt(
        this.logger,
        () => this.yieldManagerContractClient.safeMaxAddToWithdrawalReserve(this.yieldProvider),
        "_process - safeMaxAddToWithdrawalReserve failed",
      );
      await this.operationModeMetricsRecorder.recordSafeWithdrawalMetrics(this.yieldProvider, withdrawalResult);
    }
  }
}
