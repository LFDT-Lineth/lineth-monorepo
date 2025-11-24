import { ILogger, wait } from "@consensys/linea-shared-utils";
import { IYieldManager } from "../core/clients/contracts/IYieldManager.js";
import { Address, TransactionReceipt } from "viem";
import { IOperationModeSelector } from "../core/services/operation-mode/IOperationModeSelector.js";
import { IOperationModeProcessor } from "../core/services/operation-mode/IOperationModeProcessor.js";
import { INativeYieldAutomationMetricsUpdater } from "../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { OperationMode } from "../core/enums/OperationModeEnums.js";

/**
 * Selects and executes the appropriate operation mode based on the yield provider's ossification state.
 * Continuously polls the YieldManager contract to determine the current state and routes execution
 * to the corresponding operation mode processor. Handles errors with retry logic.
 */
export class OperationModeSelector implements IOperationModeSelector {
  private isRunning = false;

  /**
   * Creates a new OperationModeSelector instance.
   *
   * @param {ILogger} logger - Logger instance for logging operation mode selection and execution.
   * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating operation mode metrics.
   * @param {IYieldManager<TransactionReceipt>} yieldManagerContractClient - Client for reading yield provider state from YieldManager contract.
   * @param {IOperationModeProcessor} yieldReportingOperationModeProcessor - Processor for YIELD_REPORTING_MODE operations.
   * @param {IOperationModeProcessor} ossificationPendingOperationModeProcessor - Processor for OSSIFICATION_PENDING_MODE operations.
   * @param {IOperationModeProcessor} ossificationCompleteOperationModeProcessor - Processor for OSSIFICATION_COMPLETE_MODE operations.
   * @param {Address} yieldProvider - The yield provider address to monitor and process.
   * @param {number} contractReadRetryTimeMs - Delay in milliseconds before retrying after a contract read error.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly yieldManagerContractClient: IYieldManager<TransactionReceipt>,
    private readonly yieldReportingOperationModeProcessor: IOperationModeProcessor,
    private readonly ossificationPendingOperationModeProcessor: IOperationModeProcessor,
    private readonly ossificationCompleteOperationModeProcessor: IOperationModeProcessor,
    private readonly yieldProvider: Address,
    private readonly contractReadRetryTimeMs: number,
  ) {
  }

  /**
   * Starts the operation mode selection loop.
   * Sets the running flag and begins polling for operation mode selection.
   * If already running, returns immediately without starting a new loop.
   *
   * @returns {Promise<void>} A promise that resolves when the loop starts (but does not resolve until the loop stops).
   */
  public async start(): Promise<void> {
    if (this.isRunning) {
      return;
    }

    this.isRunning = true;
    this.logger.info(`Starting selectOperationModeLoop`);
    await this.selectOperationModeLoop();
  }

  /**
   * Stops the operation mode selection loop.
   * Sets the running flag to false, which causes the loop to exit on its next iteration.
   * If not running, returns immediately.
   */
  public stop(): void {
    if (!this.isRunning) {
      return;
    }

    this.isRunning = false;
    this.logger.info(`Stopped selectOperationModeLoop`);
  }

  /**
   * Main loop that continuously selects and executes operation modes based on yield provider state.
   * Polls the YieldManager contract to check ossification status and routes execution accordingly:
   * - If ossified: executes OSSIFICATION_COMPLETE_MODE processor
   * - Else if ossification initiated: executes OSSIFICATION_PENDING_MODE processor
   * - Otherwise: executes YIELD_REPORTING_MODE processor
   * Records metrics for each execution and handles errors with retry logic using contractReadRetryTimeMs delay.
   *
   * @returns {Promise<void>} A promise that resolves when the loop exits (when isRunning becomes false).
   */
  private async selectOperationModeLoop(): Promise<void> {
    while (this.isRunning) {
      try {
        const [isOssificationInitiated, isOssified] = await Promise.all([
          this.yieldManagerContractClient.isOssificationInitiated(this.yieldProvider),
          this.yieldManagerContractClient.isOssified(this.yieldProvider),
        ]);

        if (isOssified) {
          this.logger.info("Selected OSSIFICATION_COMPLETE_MODE");
          await this.ossificationCompleteOperationModeProcessor.process();
          this.logger.info("Completed OSSIFICATION_COMPLETE_MODE");
          this.metricsUpdater.incrementOperationModeExecution(OperationMode.OSSIFICATION_COMPLETE_MODE);
        } else if (isOssificationInitiated) {
          this.logger.info("Selected OSSIFICATION_PENDING_MODE");
          await this.ossificationPendingOperationModeProcessor.process();
          this.logger.info("Completed OSSIFICATION_PENDING_MODE");
          this.metricsUpdater.incrementOperationModeExecution(OperationMode.OSSIFICATION_PENDING_MODE);
        } else {
          this.logger.info("Selected YIELD_REPORTING_MODE");
          await this.yieldReportingOperationModeProcessor.process();
          this.logger.info("Completed YIELD_REPORTING_MODE");
          this.metricsUpdater.incrementOperationModeExecution(OperationMode.YIELD_REPORTING_MODE);
        }
      } catch (error) {
        this.logger.error(`selectOperationModeLoop error, retrying in ${this.contractReadRetryTimeMs}ms`, { error });
        await wait(this.contractReadRetryTimeMs);
      }
    }
  }
}
