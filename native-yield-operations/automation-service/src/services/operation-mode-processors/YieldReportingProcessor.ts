import { Address, TransactionReceipt } from "viem";
import { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import { IOperationModeProcessor } from "../../core/services/operation-mode/IOperationModeProcessor.js";
import {
  bigintReplacer,
  ILogger,
  attempt,
  msToSeconds,
  weiToGweiNumber,
  ONE_ETHER,
} from "@consensys/linea-shared-utils";
import { ILazyOracle } from "../../core/clients/contracts/ILazyOracle.js";
import { ILidoAccountingReportClient } from "../../core/clients/ILidoAccountingReportClient.js";
import { RebalanceDirection, RebalanceRequirement } from "../../core/entities/RebalanceRequirement.js";
import { ILineaRollupYieldExtension } from "../../core/clients/contracts/ILineaRollupYieldExtension.js";
import { IBeaconChainStakingClient } from "../../core/clients/IBeaconChainStakingClient.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";
import { IOperationModeMetricsRecorder } from "../../core/metrics/IOperationModeMetricsRecorder.js";
import { IVaultHub } from "../../core/clients/contracts/IVaultHub.js";
import { submitVaultReportIfNotFresh } from "./vaultReportSubmission.js";

/**
 * Processor for YIELD_REPORTING_MODE operations.
 * Handles native yield reporting cycles including rebalancing, vault report submission, and beacon chain withdrawals.
 */
export class YieldReportingProcessor implements IOperationModeProcessor {
  private vault: Address;
  private cycleCount: number = 0;

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
   * @param {IVaultHub<TransactionReceipt>} vaultHubContractClient - Client for interacting with VaultHub contracts.
   * @param {Address} yieldProvider - The yield provider address to process.
   * @param {Address} l2YieldRecipient - The L2 yield recipient address for yield reporting.
   * @param {boolean} shouldSubmitVaultReport - Whether to submit the vault accounting report. Can be set to false if other actors are expected to submit.
   * @param {boolean} shouldReportYield - Whether to report yield. Can be set to false to disable yield reporting entirely.
   * @param {boolean} isUnpauseStakingEnabled - Whether to unpause staking when conditions are met. Can be set to false to disable automatic unpause of staking operations.
   * @param {bigint} minNegativeYieldDiffToReportYieldWei - Minimum difference between peeked negative yield and on-state negative yield (in wei) required before triggering a yield report.
   * @param {bigint} minWithdrawalThresholdEth - Minimum withdrawal threshold in ETH (stored as wei) required before withdrawal operations proceed.
   * @param {number} cyclesPerYieldReport - Number of processing cycles between forced yield reports. Yield will be reported every N cycles regardless of threshold checks.
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
    private readonly vaultHubContractClient: IVaultHub<TransactionReceipt>,
    private readonly yieldProvider: Address,
    private readonly l2YieldRecipient: Address,
    private readonly shouldSubmitVaultReport: boolean,
    private readonly shouldReportYield: boolean,
    private readonly isUnpauseStakingEnabled: boolean,
    private readonly minNegativeYieldDiffToReportYieldWei: bigint,
    private readonly minWithdrawalThresholdEth: bigint,
    private readonly cyclesPerYieldReport: number,
  ) {
    this.cycleCount = 0;
  }

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
    await this.lazyOracleContractClient.waitForVaultsReportDataUpdatedEvent();
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
    // Increment cycle counter
    this.cycleCount++;
    this.logger.info(`_process - cycleCount incremented to ${this.cycleCount}`);
    // Fetch initial data
    this.vault = await this.yieldManagerContractClient.getLidoStakingVaultAddress(this.yieldProvider);
    // Fresh vault report is a dependency for getRebalanceRequirements
    await this._handleSubmitLatestVaultReport();
    const initialRebalanceRequirements = await this.yieldManagerContractClient.getRebalanceRequirements(
      this.yieldProvider,
      this.l2YieldRecipient,
    );
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

    const postReportRebalanceRequirements = await this.yieldManagerContractClient.getRebalanceRequirements(
      this.yieldProvider,
      this.l2YieldRecipient,
    );
    this.logger.info(
      `_process - Post rebalance data fetch: postReportRebalanceRequirements=${JSON.stringify(postReportRebalanceRequirements, bigintReplacer, 2)}`,
    );

    // Mid-cycle drift check:
    // If external flows flipped us to DEFICIT, immediately correct with a targeted UNSTAKE amendment.
    if (
      initialRebalanceRequirements.rebalanceDirection !== RebalanceDirection.UNSTAKE &&
      postReportRebalanceRequirements.rebalanceDirection === RebalanceDirection.UNSTAKE
    ) {
      await this._handleUnstakingRebalance(postReportRebalanceRequirements.rebalanceAmount, false);
    }

    // Beacon-chain withdrawals are last:
    // These have fulfillment latency beyond this method; queue them after local state is stable.
    const beaconChainWithdrawalRequirements = await this.yieldManagerContractClient.getRebalanceRequirements(
      this.yieldProvider,
      this.l2YieldRecipient,
    );
    this.logger.info(
      `_process - Beacon chain withdrawal data fetch: beaconChainWithdrawalRequirements=${JSON.stringify(beaconChainWithdrawalRequirements, bigintReplacer, 2)}`,
    );
    if (beaconChainWithdrawalRequirements.rebalanceDirection === RebalanceDirection.UNSTAKE) {
      await this.beaconChainStakingClient.submitWithdrawalRequestsToFulfilAmount(
        beaconChainWithdrawalRequirements.rebalanceAmount,
      );
    }

    // If we don't need any ETH on L1MessageService, and there is ETH sitting on StakingVault, unpause staking to allow Deposit Service to stake.
    if (
      this.isUnpauseStakingEnabled &&
      initialRebalanceRequirements.rebalanceDirection !== RebalanceDirection.UNSTAKE &&
      postReportRebalanceRequirements.rebalanceDirection !== RebalanceDirection.UNSTAKE
    ) {
      await attempt(
        this.logger,
        () => this.yieldManagerContractClient.unpauseStakingIfNotAlready(this.yieldProvider),
        "_process - unpause staking failed (tolerated)",
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
      await this._handleNoRebalance();
    } else if (rebalanceRequirements.rebalanceDirection === RebalanceDirection.STAKE) {
      await this._handleStakingRebalance(rebalanceRequirements.rebalanceAmount);
    } else {
      await this._handleUnstakingRebalance(rebalanceRequirements.rebalanceAmount, true);
    }
  }

  /**
   * Handles the no-rebalance pathway when rebalance direction is NONE.
   * Processes edge cases where funds may be sitting on the YieldManager contract and submits the vault report.
   *
   * Flow:
   * 1. Checks if YieldManager has a balance exceeding the minimum withdrawal threshold
   * 2. If threshold is met, transfers all YieldManager balance to the yield provider (tolerates failures)
   * 3. Records transfer metrics if a transfer was attempted
   * 4. Submits the latest vault report and reports yield (if thresholds are met)
   *
   * Edge case handling:
   * - Funds may accumulate on YieldManager from various sources (e.g., refunds, fees)
   * - These funds should be transferred to the yield provider to maintain proper accounting
   * - Only transfers if balance exceeds MIN_WITHDRAWAL_THRESHOLD_ETH to avoid gas-inefficient transactions
   *
   * @returns {Promise<void>} A promise that resolves when the no-rebalance pathway is handled.
   */
  private async _handleNoRebalance(): Promise<void> {
    // Handle edge case where funds on the YieldManager
    const [yieldManagerBalance, targetReserveDeficit] = await Promise.all([
      this.yieldManagerContractClient.getBalance(),
      this.yieldManagerContractClient.getTargetReserveDeficit(),
    ]);
    if (yieldManagerBalance > this.minWithdrawalThresholdEth * ONE_ETHER) {
      // Must amend target deficit or else fundYieldProvider reverts
      if (targetReserveDeficit > 0n) {
        const withdrawalResult = await attempt(
          this.logger,
          () => this.yieldManagerContractClient.safeWithdrawFromYieldProvider(this.yieldProvider, targetReserveDeficit),
          "_handleNoRebalance - safeWithdrawFromYieldProvider failed (tolerated)",
        );
        await this.operationModeMetricsRecorder.recordSafeWithdrawalMetrics(this.yieldProvider, withdrawalResult);
      }

      const transferFundsResult = await attempt(
        this.logger,
        () => this.yieldManagerContractClient.fundYieldProvider(this.yieldProvider, yieldManagerBalance),
        "_handleStakingRebalance - fundYieldProvider failed (tolerated)",
      );
      await this.operationModeMetricsRecorder.recordTransferFundsMetrics(this.yieldProvider, transferFundsResult);
    }

    this.logger.info("_handleNoRebalance - no rebalance pathway, calling _handleReportYield");
    await this._handleReportYield();
    return;
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
    this.logger.info("_handleStakingRebalance calling _handleReportYield");
    await this._handleReportYield();
  }

  /**
   * Handles unstaking rebalance operations when there is a reserve deficit.
   * Submit report first (if yield should be reported), then perform rebalance.
   *
   * @param {bigint} rebalanceAmount - The amount to rebalance in wei.
   * @param {boolean} shouldReportYield - Whether to report yield before rebalancing. If true, submits vault report before rebalancing.
   * @returns {Promise<void>} A promise that resolves when unstaking rebalance is handled.
   */
  private async _handleUnstakingRebalance(rebalanceAmount: bigint, shouldReportYield: boolean): Promise<void> {
    if (shouldReportYield) {
      // Submit report first
      this.logger.info("_handleUnstakingRebalance calling _handleReportYield");
      await this._handleReportYield();
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
   * @dev Checks if report is fresh before submitting to avoid unnecessary transactions.
   * @returns {Promise<void>} A promise that resolves when both operations are attempted (regardless of success/failure).
   */
  private async _handleSubmitLatestVaultReport() {
    await submitVaultReportIfNotFresh(
      this.logger,
      this.vaultHubContractClient,
      this.lidoAccountingReportClient,
      this.metricsUpdater,
      this.vault,
      this.shouldSubmitVaultReport,
      "_handleSubmitLatestVaultReport",
    );
  }

  async _handleReportYield(): Promise<void> {
    if (await this._shouldReportYield()) {
      const yieldResult = await attempt(
        this.logger,
        () => this.yieldManagerContractClient.reportYield(this.yieldProvider, this.l2YieldRecipient),
        "_handleReportYield - reportYield failed",
      );
      if (yieldResult.isOk()) {
        this.logger.info("_handleReportYield: yield report succeeded");
        await this.operationModeMetricsRecorder.recordReportYieldMetrics(this.yieldProvider, yieldResult);
      }
    }
  }

  /**
   * Determines whether yield should be reported based on configurable thresholds and cycle count.
   * Checks the negative yield difference against its threshold,
   * and cycle-based reporting (every N cycles).
   * Returns true if any threshold is met or exceeded, or if cycle-based reporting is due.
   * Sets gauge metrics for peeked values when reads are successful.
   *
   * @returns {Promise<boolean>} True if yield should be reported (any threshold met), false otherwise.
   */
  async _shouldReportYield(): Promise<boolean> {
    // Use Promise.all to concurrently fetch values
    const [settleableLidoFees, yieldReport, yieldProviderData] = await Promise.all([
      this.vaultHubContractClient.settleableLidoFeesValue(this.vault),
      this.yieldManagerContractClient.peekYieldReport(this.yieldProvider, this.l2YieldRecipient),
      this.yieldManagerContractClient.getYieldProviderData(this.yieldProvider),
    ]);

    let negativeYieldDiffThresholdMet = false;
    let isCycleBasedReportingDue = false;

    if (yieldReport !== undefined) {
      const outstandingNegativeYield = yieldReport?.outstandingNegativeYield;
      const yieldAmount = yieldReport?.yieldAmount;
      this.metricsUpdater.setLastPeekedNegativeYieldReport(this.vault, weiToGweiNumber(outstandingNegativeYield));
      this.metricsUpdater.setLastPeekedPositiveYieldReport(this.vault, weiToGweiNumber(yieldAmount));

      // Check negative yield difference threshold
      if (yieldProviderData !== undefined) {
        const onStateNegativeYield = yieldProviderData.lastReportedNegativeYield;
        const negativeYieldDiff = outstandingNegativeYield - onStateNegativeYield;
        negativeYieldDiffThresholdMet = negativeYieldDiff >= this.minNegativeYieldDiffToReportYieldWei;
      }
    }

    if (settleableLidoFees !== undefined) {
      this.metricsUpdater.setLastSettleableLidoFees(this.vault, weiToGweiNumber(settleableLidoFees));
    }

    // Check if cycle-based reporting is due
    isCycleBasedReportingDue = this.cycleCount % this.cyclesPerYieldReport === 0;

    const shouldReportYield = this.shouldReportYield && (negativeYieldDiffThresholdMet || isCycleBasedReportingDue);

    // Log results
    const onStateNegativeYield = yieldProviderData?.lastReportedNegativeYield;
    const peekedNegativeYield = yieldReport?.outstandingNegativeYield;
    const negativeYieldDiff =
      peekedNegativeYield !== undefined && onStateNegativeYield !== undefined
        ? peekedNegativeYield - onStateNegativeYield
        : undefined;
    this.logger.info(
      `_shouldReportYield - shouldReportYield=${shouldReportYield}, cycleCount=${this.cycleCount}, cycleBasedReportingDue=${isCycleBasedReportingDue}, settleableLidoFees=${JSON.stringify(settleableLidoFees, bigintReplacer)}, yieldReport=${JSON.stringify(yieldReport, bigintReplacer)}, onStateNegativeYield=${JSON.stringify(onStateNegativeYield, bigintReplacer)}, negativeYieldDiff=${JSON.stringify(negativeYieldDiff, bigintReplacer)}`,
    );

    // Return true if any threshold is met
    return shouldReportYield;
  }
}
