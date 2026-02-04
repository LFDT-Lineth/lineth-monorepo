import { Address } from "viem";
import { RebalanceDirection } from "../entities/RebalanceRequirement.js";

/**
 * Interface for service to handle enforcing rebalance quota.
 * Enforces a configurable deposit quota for StakingVault deposits to mitigate whale-driven reserve depletion risk.
 * The quota is calculated as a percentage of Total System Balance (TSB) over a configurable rolling time window.
 *
 * @remarks
 * - Quota percentage and time window duration are configurable via constructor parameters
 * - Setting quotaWindowSizeInCycles to 0 disables the quota mechanism entirely - all amounts pass through
 * - Returns 0 when quota is exceeded to halt deposits completely
 * - Called during STAKE direction rebalance processing in YieldManagerContractClient
 */
export interface IRebalanceQuotaService {
  /**
   * Processes a rebalance amount through the quota check and returns the amount after applying quota limits.
   * Checks cumulative deposits over a configurable rolling time window and compares against a configurable
   * percentage of Total System Balance (TSB).
   *
   * @param vaultAddress - The address of the vault, used for metrics tracking.
   * @param totalSystemBalance - The total system balance (TSB) in wei, used to calculate the quota threshold.
   * @param reBalanceAmountWei - The rebalance amount to be processed (in wei).
   * @returns The actual amount to be rebalanced (in wei) after quota check.
   *          If quota mechanism is disabled (quotaWindowSizeInCycles = 0), returns the full reBalanceAmountWei without quota enforcement.
   *          Otherwise, returns 0n if:
   *            - The amount is below the tolerance threshold, or
   *            - Adding this amount would exceed the quota and previous cycle was also over quota.
   *          Returns the original reBalanceAmountWei if within quota (newTotal <= quotaWei).
   *          Returns a partial amount (quotaWei - prevTotal) if crossing the quota threshold but previous cycle was under quota.
   */
  getRebalanceAmountAfterQuota(vaultAddress: Address, totalSystemBalance: bigint, reBalanceAmountWei: bigint): bigint;

  /**
   * Returns the rebalance direction this quota service applies to.
   * Makes the interface agnostic to direction, allowing future extensions for other directions.
   *
   * @returns The rebalance direction this quota service enforces (e.g., RebalanceDirection.STAKE).
   */
  getStakingDirection(): RebalanceDirection;
}
