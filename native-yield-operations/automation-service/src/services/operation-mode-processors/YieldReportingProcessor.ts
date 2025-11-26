import { Address, TransactionReceipt } from "viem";
import { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import { IOperationModeProcessor } from "../../core/services/operation-mode/IOperationModeProcessor.js";
import { bigintReplacer, ILogger, attempt, msToSeconds, weiToGweiNumber } from "@consensys/linea-shared-utils";
import { ILazyOracle } from "../../core/clients/contracts/ILazyOracle.js";
import { ILidoAccountingReportClient } from "../../core/clients/ILidoAccountingReportClient.js";
import { RebalanceDirection, RebalanceRequirement } from "../../core/entities/RebalanceRequirement.js";
import { ILineaRollupYieldExtension } from "../../core/clients/contracts/ILineaRollupYieldExtension.js";
import { IBeaconChainStakingClient } from "../../core/clients/IBeaconChainStakingClient.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";
import { IOperationModeMetricsRecorder } from "../../core/metrics/IOperationModeMetricsRecorder.js";
import { DashboardContractClient } from "../../clients/contracts/DashboardContractClient.js";

/**
 * Processor for YIELD_REPORTING_MODE operations.
 * Handles native yield reporting cycles including rebalancing, vault report submission, and beacon chain withdrawals.
 */
export class YieldReportingProcessor implements IOperationModeProcessor {
  private vault: Address;

  /**
   * Creates a new YieldReportingProcessor instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating operation mode metrics.
   * @param {IOperationModeMetricsRecorder} operationModeMetricsRecorder - Service for recording operation mode metrics from transaction receipts.
   * @param {IYieldManager<TransactionReceipt>} yieldManagerContractClient - Client for interacting with YieldManager contracts.
   * @param {ILazyOracle<TransactionReceipt>} lazyOracleContractClient - Client for waiting on LazyOracle events.
   * @param {ILineaRollupYieldExtension<TransactionReceipt>} lineaRollupYieldExtensionClient - Client for interacting with LineaRollupYieldExtension contracts.
   * @param {ILidoAccountingReportClient} lidoAccountingReportClient - Client for submitting Lido accounting reports.
   * @param {IBeaconChainStakingClient} beaconChainStakingClient - Client for managing beacon chain staking operations.
   * @param {Address} yieldProvider - The yield provider address to process.
   * @param {Address} l2YieldRecipient - The L2 yield recipient address for yield reporting.
   * @param {boolean} shouldSubmitVaultReport - Whether to submit the vault accounting report. Can be set to false if other actors are expected to submit.
   * @param {bigint} minPositiveYieldToReportWei - Minimum positive yield amount (in wei) required before triggering a yield report.
   * @param {bigint} minUnpaidLidoProtocolFeesToReportYieldWei - Minimum unpaid Lido protocol fees amount (in wei) required before triggering a fee settlement.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly operationModeMetricsRecorder: IOperationModeMetricsRecorder,
    private readonly yieldManagerContractClient: IYieldManager<TransactionReceipt>,
    private readonly lazyOracleContractClient: ILazyOracle<TransactionReceipt>,
    private readonly lineaRollupYieldExtensionClient: ILineaRollupYieldExtension<TransactionReceipt>,
    private readonly lidoAccountingReportClient: ILidoAccountingReportClient,
    private readonly beaconChainStakingClient: IBeaconChainStakingClient,
    private readonly yieldProvider: Address,
    private readonly l2YieldRecipient: Address,
    private readonly shouldSubmitVaultReport: boolean,
    private readonly minPositiveYieldToReportWei: bigint,
    private readonly minUnpaidLidoProtocolFeesToReportYieldWei: bigint,
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
    this.metricsUpdater.incrementOperationModeTrigger(OperationMode.YIELD_REPORTING_MODE, triggerEvent.result);
    const startedAt = performance.now();
    await this._process();
    const durationMs = performance.now() - startedAt;
    this.metricsUpdater.recordOperationModeDuration(OperationMode.YIELD_REPORTING_MODE, msToSeconds(durationMs));
  }

  /**
   * Orchestrates a single Native Yield reporting cycle.
   *
   * High-level flow
   *  1) Read current state:
   *     - Fetch latest Lido report params
   *     - Determine rebalance direction/amount (stake vs. unstake vs. no-op)
   *  2) Front-running / safety:
   *     - If we're in DEFICIT (need to UNSTAKE), pause staking up-front to
   *       prevent new deposits worsening the shortfall while we act
   *  3) Primary action:
   *     - Perform the primary rebalance and (if possible) submit the report
   *  4) Mid-cycle drift fix:
   *     - If we *started* needing STAKE but, during processing, external flows
   *       (e.g., bridge withdrawals) flipped us into DEFICIT, perform an
   *       *amendment* UNSTAKE to restore reserve targets
   *  5) Resume normal operations:
   *     - If we started in EXCESS (STAKE) and did *not* end in DEFICIT, unpause staking
   *  6) Beacon chain withdrawals:
   *     - Defer actual validator withdrawals to the very end since fulfillment
   *       extends beyond this method’s synchronous runtime
   *
   * Invariants and intent:
   *  - Never leave staking enabled while in a known deficit
   *  - Prefer to submit the accounting report in the same cycle as the primary rebalance
   *  - Make at most one amendment pass to handle mid-cycle state changes
   *
   * Side effects:
   *  - May pause/unpause staking
   *  - May submit Lido vault report
   *  - May stake/unstake on the vault
   *  - May queue beacon-chain withdrawals
   */
  private async _process(): Promise<void> {
    // Fetch initial data
    this.vault = await this.yieldManagerContractClient.getLidoStakingVaultAddress(this.yieldProvider);
    const [initialRebalanceRequirements] = await Promise.all([
      this.yieldManagerContractClient.getRebalanceRequirements(),
      this.shouldSubmitVaultReport
        ? this.lidoAccountingReportClient.getLatestSubmitVaultReportParams(this.vault)
        : Promise.resolve(),
    ]);
    this.logger.info(
      `_process - Initial data fetch: initialRebalanceRequirements=${JSON.stringify(initialRebalanceRequirements, bigintReplacer, 2)}`,
    );

    // If we begin in DEFICIT, freeze beacon chain deposits to prevent further exacerbation
    if (initialRebalanceRequirements.rebalanceDirection === RebalanceDirection.UNSTAKE) {
      await attempt(
        this.logger,
        () => this.yieldManagerContractClient.pauseStakingIfNotAlready(this.yieldProvider),
        "_process - pause staking failed (tolerated)",
      );
    }

    // Do primary rebalance +/- report submission
    await this._handleRebalance(initialRebalanceRequirements);

    const postReportRebalanceRequirements = await this.yieldManagerContractClient.getRebalanceRequirements();
    this.logger.info(
      `_process - Post rebalance data fetch: postReportRebalanceRequirements=${JSON.stringify(postReportRebalanceRequirements, bigintReplacer, 2)}`,
    );

    // Mid-cycle drift check:
    // If we *started* with EXCESS (STAKE) but external flows flipped us to DEFICIT,
    // immediately correct with a targeted UNSTAKE amendment.
    if (initialRebalanceRequirements.rebalanceDirection === RebalanceDirection.STAKE) {
      if (postReportRebalanceRequirements.rebalanceDirection === RebalanceDirection.UNSTAKE) {
        await this._handleUnstakingRebalance(postReportRebalanceRequirements.rebalanceAmount, false);
      } else {
        await attempt(
          this.logger,
          () => this.yieldManagerContractClient.unpauseStakingIfNotAlready(this.yieldProvider),
          "_process - unpause staking failed (tolerated)",
        );
      }
    }

    // Beacon-chain withdrawals are last:
    // These have fulfillment latency beyond this method; queue them after local state is stable.
    const beaconChainWithdrawalRequirements = await this.yieldManagerContractClient.getRebalanceRequirements();
    this.logger.info(
      `_process - Beacon chain withdrawal data fetch: beaconChainWithdrawalRequirements=${JSON.stringify(beaconChainWithdrawalRequirements, bigintReplacer, 2)}`,
    );
    if (beaconChainWithdrawalRequirements.rebalanceDirection === RebalanceDirection.UNSTAKE) {
      await this.beaconChainStakingClient.submitWithdrawalRequestsToFulfilAmount(
        beaconChainWithdrawalRequirements.rebalanceAmount,
      );
    }
  }

  /**
   * Handles rebalancing operations based on rebalance requirements.
   * Routes to appropriate handler based on rebalance direction:
   * - NONE: Simple submit report
   * - STAKE: Handles staking rebalance (surplus)
   * - UNSTAKE: Handles unstaking rebalance (deficit)
   *
   * @param {RebalanceRequirement} rebalanceRequirements - The rebalance requirements containing direction and amount.
   * @returns {Promise<void>} A promise that resolves when rebalancing is handled.
   */
  private async _handleRebalance(rebalanceRequirements: RebalanceRequirement): Promise<void> {
    if (rebalanceRequirements.rebalanceDirection === RebalanceDirection.NONE) {
      // No-op
      this.logger.info("_handleRebalance - no rebalance pathway, calling _handleSubmitLatestVaultReport");
      await this._handleSubmitLatestVaultReport();
      return;
    } else if (rebalanceRequirements.rebalanceDirection === RebalanceDirection.STAKE) {
      await this._handleStakingRebalance(rebalanceRequirements.rebalanceAmount);
    } else {
      await this._handleUnstakingRebalance(rebalanceRequirements.rebalanceAmount, true);
    }
  }

  /**
   * Handles staking rebalance operations when there is a reserve surplus.
   * Rebalance first - tolerate failures because fresh vault report should not be blocked.
   * Only do YieldManager->YieldProvider, if L1MessageService->YieldManager succeeded.
   * Submit report last.
   *
   * Assumptions:
   * - i.) We count rebalance once funds have been moved away from the L1MessageService
   * - ii.) Only the initial rebalance will call this fn
   *
   * @param {bigint} rebalanceAmount - The amount to rebalance in wei.
   * @returns {Promise<void>} A promise that resolves when staking rebalance is handled.
   */
  // Surplus
  private async _handleStakingRebalance(rebalanceAmount: bigint): Promise<void> {
    this.logger.info(`_handleStakingRebalance - reserve surplus, rebalanceAmount=${rebalanceAmount}`);
    // Rebalance first - tolerate failures because fresh vault report should not be blocked
    const transferFundsForNativeYieldResult = await attempt(
      this.logger,
      () => this.lineaRollupYieldExtensionClient.transferFundsForNativeYield(rebalanceAmount),
      "_handleStakingRebalance - transferFundsForNativeYield failed (tolerated)",
    );
    // Only do YieldManager->YieldProvider, if L1MessageService->YieldManager succeeded
    if (transferFundsForNativeYieldResult.isOk()) {
      // Assumptions
      // i.) We count rebalance once funds have been moved away from the L1MessageService
      // ii.) Only the initial rebalance will call this fn
      this.metricsUpdater.recordRebalance(RebalanceDirection.STAKE, weiToGweiNumber(rebalanceAmount));
      const transferFundsResult = await attempt(
        this.logger,
        () => this.yieldManagerContractClient.fundYieldProvider(this.yieldProvider, rebalanceAmount),
        "_handleStakingRebalance - fundYieldProvider failed (tolerated)",
      );
      await this.operationModeMetricsRecorder.recordTransferFundsMetrics(this.yieldProvider, transferFundsResult);
    }

    // Submit report last
    this.logger.info("_handleStakingRebalance calling _handleSubmitLatestVaultReport");
    await this._handleSubmitLatestVaultReport();
  }

  /**
   * Handles unstaking rebalance operations when there is a reserve deficit.
   * Submit report first (if yield should be reported), then perform rebalance.
   *
   * @param {bigint} rebalanceAmount - The amount to rebalance in wei.
   * @param {boolean} shouldReportYield - Whether to report yield before rebalancing. If true, submits vault report before rebalancing.
   * @returns {Promise<void>} A promise that resolves when unstaking rebalance is handled.
   */
  // Deficit
  private async _handleUnstakingRebalance(rebalanceAmount: bigint, shouldReportYield: boolean): Promise<void> {
    if (shouldReportYield) {
      // Submit report first
      this.logger.info("_handleUnstakingRebalance calling _handleSubmitLatestVaultReport");
      await this._handleSubmitLatestVaultReport();
    }

    this.logger.info(`_handleUnstakingRebalance - reserve deficit, rebalanceAmount=${rebalanceAmount}`);
    // Then perform rebalance
    const withdrawalResult = await attempt(
      this.logger,
      () =>
        this.yieldManagerContractClient.safeAddToWithdrawalReserveIfAboveThreshold(this.yieldProvider, rebalanceAmount),
      "_handleUnstakingRebalance - safeAddToWithdrawalReserveIfAboveThreshold failed (tolerated)",
    );
    await this.operationModeMetricsRecorder.recordSafeWithdrawalMetrics(this.yieldProvider, withdrawalResult);
  }

  /**
   * @notice Submits the latest vault report (if enabled) and then reports yield to the yield manager.
   * @dev Uses `tryResult` to safely handle failures without throwing.
   *      - If submitting the vault report fails, the execution will continue to report yield.
   *        A key assumption is that it is safe to submit multiple yield reports for the same vault report.
   * @dev We tolerate report submission errors because they should not block rebalances
   * @dev If shouldSubmitVaultReport is false, skips vault report submission but still reports yield.
   * @returns {Promise<void>} A promise that resolves when both operations are attempted (regardless of success/failure).
   */
  private async _handleSubmitLatestVaultReport() {
    // First call: submit vault report (if enabled)
    if (this.shouldSubmitVaultReport) {
      const vaultResult = await attempt(
        this.logger,
        () => this.lidoAccountingReportClient.submitLatestVaultReport(this.vault),
        "_handleSubmitLatestVaultReport: submitLatestVaultReport failed",
      );
      if (vaultResult.isOk()) {
        this.logger.info("_handleSubmitLatestVaultReport: vault report succeeded");
        this.metricsUpdater.incrementLidoVaultAccountingReport(this.vault);
      }
    } else {
      this.logger.info(
        "_handleSubmitLatestVaultReport: skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)",
      );
    }

    // Second call: report yield
    if (await this._shouldReportYield()) {
      const yieldResult = await attempt(
        this.logger,
        () => this.yieldManagerContractClient.reportYield(this.yieldProvider, this.l2YieldRecipient),
        "_handleSubmitLatestVaultReport - reportYield failed",
      );
      if (yieldResult.isOk()) {
        this.logger.info("_handleSubmitLatestVaultReport: yield report succeeded");
        await this.operationModeMetricsRecorder.recordReportYieldMetrics(this.yieldProvider, yieldResult);
      }
    }
  }

  /**
   * Determines whether yield should be reported based on configurable thresholds.
   * Checks both the yield amount and unpaid Lido protocol fees against their respective thresholds.
   * Returns true if either threshold is met or exceeded.
   * Sets gauge metrics for peeked values when reads are successful.
   *
   * @returns {Promise<boolean>} True if yield should be reported (either threshold met), false otherwise.
   */
  async _shouldReportYield(): Promise<boolean> {
    // Get dashboard address
    const dashboardAddress = await this.yieldManagerContractClient.getLidoDashboardAddress(this.yieldProvider);
    const dashboardClient = DashboardContractClient.getOrCreate(dashboardAddress);

    // Use Promise.all to concurrently fetch both values
    const [unpaidLidoProtocolFees, yieldReport] = await Promise.all([
      dashboardClient.peekUnpaidLidoProtocolFees(),
      this.yieldManagerContractClient.peekYieldReport(this.yieldProvider, this.l2YieldRecipient),
    ]);

    // Log both results
    this.logger.info(
      `_shouldReportYield - unpaidLidoProtocolFees=${JSON.stringify(unpaidLidoProtocolFees, bigintReplacer)}, yieldReport=${JSON.stringify(yieldReport, bigintReplacer)}`,
    );

    let yieldThresholdMet = false;
    let feesThresholdMet = false;

    if (yieldReport !== undefined) {
      const outstandingNegativeYield = yieldReport?.outstandingNegativeYield;
      const yieldAmount = yieldReport?.yieldAmount;
      await Promise.all([
        this.metricsUpdater.setLastPeekedNegativeYieldReport(this.vault, weiToGweiNumber(outstandingNegativeYield)),
        this.metricsUpdater.setLastPeekedPositiveYieldReport(this.vault, weiToGweiNumber(yieldAmount)),
      ]);
      yieldThresholdMet = yieldAmount >= this.minPositiveYieldToReportWei;
    }

    if (unpaidLidoProtocolFees !== undefined) {
      await this.metricsUpdater.setLastPeekUnpaidLidoProtocolFees(this.vault, weiToGweiNumber(unpaidLidoProtocolFees));
      feesThresholdMet = unpaidLidoProtocolFees >= this.minUnpaidLidoProtocolFeesToReportYieldWei;
    }

    // Return true if either threshold is met
    return feesThresholdMet || yieldThresholdMet;
  }
}
