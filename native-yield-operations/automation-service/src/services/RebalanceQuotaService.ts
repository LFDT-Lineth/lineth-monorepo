import { ILogger } from "@consensys/linea-shared-utils";
import { Address } from "viem";
import { RebalanceDirection } from "../core/entities/RebalanceRequirement.js";
import { IRebalanceQuotaService } from "../core/services/IRebalanceQuotaService.js";
import { INativeYieldAutomationMetricsUpdater } from "../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { SlidingWindowAccumulator } from "../utils/SlidingWindowAccumulator.js";

/**
 * Implementation of IRebalanceQuotaService that enforces rebalance quota limits
 * using a circular buffer to track cumulative deposits over a rolling window of cycles.
 *
 * Each cycle corresponds to a YieldReportingProcessor.process() call.
 * The service tracks rebalance amounts and prevents deposits when the quota is exceeded.
 * Setting quotaWindowSizeInCycles to 0 disables the quota mechanism entirely.
 */
export class RebalanceQuotaService implements IRebalanceQuotaService {
  private readonly buffer: SlidingWindowAccumulator;
  private readonly quotaWindowSizeInCycles: number;

  /**
   * Creates a new RebalanceQuotaService instance.
   *
   * @param logger - Logger instance for logging operations.
   * @param metricsUpdater - Metrics updater for tracking quota-related metrics.
   * @param stakingDirection - The rebalance direction this quota service applies to (e.g., RebalanceDirection.STAKE).
   * @param quotaWindowSizeInCycles - The number of cycles in the rolling window (e.g., 24 for a 24-cycle window).
   *                                  Set to 0 to disable quota enforcement entirely - all amounts will pass through.
   * @param rebalanceQuotaBps - The quota as basis points (bps) of Total System Balance. 100 bps = 1%, 1800 bps = 18%.
   * @param rebalanceToleranceAmountWei - Rebalance tolerance amount in wei. Amounts below this threshold are not tracked in quota and return 0.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly stakingDirection: RebalanceDirection,
    quotaWindowSizeInCycles: number,
    private readonly rebalanceQuotaBps: number,
    private readonly rebalanceToleranceAmountWei: bigint,
  ) {
    this.quotaWindowSizeInCycles = quotaWindowSizeInCycles;
    this.buffer = new SlidingWindowAccumulator(quotaWindowSizeInCycles);
    this.logger.info(
      `RebalanceQuotaService initialized - stakingDirection=${stakingDirection}, quotaWindowSizeInCycles=${quotaWindowSizeInCycles}, rebalanceQuotaBps=${rebalanceQuotaBps}, rebalanceToleranceAmountWei=${rebalanceToleranceAmountWei.toString()}`,
    );
  }

  /**
   * Processes a rebalance amount through the quota check and returns the amount after applying quota limits.
   * Checks cumulative deposits over the rolling window and compares against the quota calculated as a percentage
   * of Total System Balance (TSB).
   *
   * @param vaultAddress - The address of the vault, used for metrics tracking.
   * @param totalSystemBalance - The total system balance (TSB) in wei, used to calculate the quota threshold.
   * @param reBalanceAmountWei - The rebalance amount to be processed (in wei).
   * @returns The actual amount to be rebalanced (in wei) after quota check.
   *          If quotaWindowSizeInCycles is 0, returns the full reBalanceAmountWei without any quota enforcement.
   *          Otherwise, returns 0n if:
   *            - The amount is below the tolerance threshold, or
   *            - Adding this amount would exceed the quota and previous cycle was also over quota.
   *          Returns the original reBalanceAmountWei if within quota (newTotal <= quotaWei).
   *          Returns a partial amount (quotaWei - prevTotal) if crossing the quota threshold but previous cycle was under quota.
   */
  getRebalanceAmountAfterQuota(vaultAddress: Address, totalSystemBalance: bigint, reBalanceAmountWei: bigint): bigint {
    // If quota mechanism is disabled (window size = 0), return full amount without quota checking
    if (this.quotaWindowSizeInCycles === 0) {
      return reBalanceAmountWei;
    }

    // If amount is below tolerance threshold, don't track in quota and return 0
    if (reBalanceAmountWei < this.rebalanceToleranceAmountWei) {
      this.logger.info(
        `getRebalanceAmountAfterQuota - below tolerance threshold: reBalanceAmountWei=${reBalanceAmountWei.toString()}, rebalanceToleranceAmountWei=${this.rebalanceToleranceAmountWei.toString()}`,
      );
      this.buffer.push(0n);
      return 0n;
    }

    const quotaWei = (totalSystemBalance * BigInt(this.rebalanceQuotaBps)) / 10000n;
    const prevTotal = this.buffer.getTotal();
    this.logger.info(
      `getRebalanceAmountAfterQuota - totalSystemBalance=${totalSystemBalance.toString()}, reBalanceAmountWei=${reBalanceAmountWei.toString()}, quotaWei=${quotaWei.toString()}, prevTotal=${prevTotal.toString()}`,
    );

    this.buffer.push(reBalanceAmountWei);
    const newTotal = this.buffer.getTotal();
    this.logger.info(`getRebalanceAmountAfterQuota - newTotal=${newTotal.toString()}`);

    // Happy path - have not hit quota
    if (newTotal <= quotaWei) return reBalanceAmountWei;
    this.metricsUpdater.incrementStakingDepositQuotaExceeded(vaultAddress);
    // Over quota, and previous cycle was also over quota -> No rebalance
    if (prevTotal >= quotaWei) {
      return 0n;
      // Over quota, but previous cycle was not over quota -> Rebalance will be limited by quota
    } else {
      return quotaWei - prevTotal;
    }
  }

  /**
   * Returns the rebalance direction this quota service applies to.
   *
   * @returns The rebalance direction this quota service enforces.
   */
  getStakingDirection(): RebalanceDirection {
    return this.stakingDirection;
  }
}
