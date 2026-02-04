import { Address, TransactionReceipt } from "viem";
import { ILogger, attempt, msToSeconds, wait } from "@consensys/linea-shared-utils";
import { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import { IOperationModeProcessor } from "../../core/services/operation-mode/IOperationModeProcessor.js";
import { IBeaconChainStakingClient } from "../../core/clients/IBeaconChainStakingClient.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";
import { IOperationModeMetricsRecorder } from "../../core/metrics/IOperationModeMetricsRecorder.js";

/**
 * Processor for OSSIFICATION_COMPLETE_MODE operations.
 * Handles ossification complete state by performing max withdrawal from yield provider
 * and max unstake from beacon chain after a configurable delay period.
 */
export class OssificationCompleteProcessor implements IOperationModeProcessor {
  /**
   * Creates a new OssificationCompleteProcessor instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating operation mode metrics.
   * @param {IOperationModeMetricsRecorder} operationModeMetricsRecorder - Service for recording operation mode metrics from transaction receipts.
   * @param {IYieldManager<TransactionReceipt>} yieldManagerContractClient - Client for interacting with YieldManager contracts.
   * @param {IBeaconChainStakingClient} beaconChainStakingClient - Client for managing beacon chain staking operations.
   * @param {number} maxInactionMs - Maximum inaction delay in milliseconds before executing actions (timeout-based trigger).
   * @param {Address} yieldProvider - The yield provider address to process.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly operationModeMetricsRecorder: IOperationModeMetricsRecorder,
    private readonly yieldManagerContractClient: IYieldManager<TransactionReceipt>,
    private readonly beaconChainStakingClient: IBeaconChainStakingClient,
    private readonly maxInactionMs: number,
    private readonly yieldProvider: Address,
  ) {}

  /**
   * Executes one processing cycle:
   * - Waits for the configured max inaction delay period.
   * - Records operation mode trigger metrics with TIMEOUT trigger.
   * - Runs the main processing logic (`_process()`).
   * - Records operation mode execution duration metrics.
   *
   * @returns {Promise<void>} A promise that resolves when the processing cycle completes.
   */
  public async process(): Promise<void> {
    this.logger.info(`Waiting ${this.maxInactionMs}ms before executing actions`);
    await wait(this.maxInactionMs);

    const startedAt = performance.now();
    await this._process();
    const durationMs = performance.now() - startedAt;
    this.metricsUpdater.recordOperationModeDuration(OperationMode.OSSIFICATION_COMPLETE_MODE, msToSeconds(durationMs));
  }

  /**
   * Main processing logic for ossification complete mode:
   * 1. Max withdraw - Performs maximum safe withdrawal from yield provider to withdrawal reserve
   * 2. Max unstake - Submits maximum available withdrawal requests from beacon chain
   *
   * @returns {Promise<void>} A promise that resolves when processing completes.
   */
  private async _process(): Promise<void> {
    // Max withdraw
    this.logger.info("_process - Performing max withdrawal from YieldProvider");
    const withdrawalResult = await attempt(
      this.logger,
      () => this.yieldManagerContractClient.safeMaxAddToWithdrawalReserve(this.yieldProvider),
      "_process - safeMaxAddToWithdrawalReserve failed (tolerated)",
    );
    await this.operationModeMetricsRecorder.recordSafeWithdrawalMetrics(this.yieldProvider, withdrawalResult);

    // Max unstake
    this.logger.info("_process - Performing max unstake from beacon chain");
    await this.beaconChainStakingClient.submitMaxAvailableWithdrawalRequests();
  }
}
