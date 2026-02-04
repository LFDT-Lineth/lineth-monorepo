import {
  ILogger,
  min,
  MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE,
  ONE_GWEI,
  safeSub,
  weiToGweiNumber,
} from "@consensys/linea-shared-utils";
import { IBeaconChainStakingClient } from "../core/clients/IBeaconChainStakingClient.js";
import { IValidatorDataClient } from "../core/clients/IValidatorDataClient.js";
import { ValidatorBalanceWithPendingWithdrawal } from "../core/entities/Validator.js";
import { WithdrawalRequests } from "../core/entities/LidoStakingVaultWithdrawalParams.js";
import { Address, Hex, maxUint256, TransactionReceipt } from "viem";
import { IYieldManager } from "../core/clients/contracts/IYieldManager.js";
import { INativeYieldAutomationMetricsUpdater } from "../core/metrics/INativeYieldAutomationMetricsUpdater.js";

/**
 * Client for managing beacon chain staking operations including withdrawal requests and validator exits.
 * Handles partial withdrawal requests up to a configured maximum per transaction and tracks metrics.
 */
export class BeaconChainStakingClient implements IBeaconChainStakingClient {
  /**
   * Creates a new BeaconChainStakingClient instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating metrics.
   * @param {IValidatorDataClient} validatorDataClient - Client for retrieving validator data.
   * @param {number} maxValidatorWithdrawalRequestsPerTransaction - Maximum number of withdrawal requests allowed per transaction.
   * @param {IYieldManager<TransactionReceipt>} yieldManagerContractClient - Client for interacting with YieldManager contracts.
   * @param {Address} yieldProvider - The yield provider address.
   * @param {bigint} minWithdrawalThresholdEth - Minimum withdrawal threshold in ETH. Withdrawal requests below this threshold will be filtered out.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly validatorDataClient: IValidatorDataClient,
    private readonly maxValidatorWithdrawalRequestsPerTransaction: number,
    private readonly yieldManagerContractClient: IYieldManager<TransactionReceipt>,
    private readonly yieldProvider: Address,
    private readonly minWithdrawalThresholdEth: bigint,
  ) {}

  /**
   * Submits withdrawal requests to fulfill a specific amount.
   * Calculates the remaining withdrawal amount needed after accounting for existing pending partial withdrawals,
   * then submits partial withdrawal requests for validators to meet the target amount.
   *
   * @param {bigint} amountWei - The target withdrawal amount in wei.
   * @returns {Promise<void>} A promise that resolves when withdrawal requests are submitted (or silently returns if validator list is unavailable).
   */
  async submitWithdrawalRequestsToFulfilAmount(amountWei: bigint): Promise<void> {
    this.logger.info(
      `submitWithdrawalRequestsToFulfilAmount started: amountWei=${amountWei.toString()}; validatorLimit=${this.maxValidatorWithdrawalRequestsPerTransaction}`,
    );
    const sortedValidatorList = await this.validatorDataClient.getValidatorsForWithdrawalRequestsAscending();
    if (sortedValidatorList === undefined) {
      this.logger.error(
        "submitWithdrawalRequestsToFulfilAmount failed to get sortedValidatorList with pending withdrawals",
      );
      return;
    }
    const totalPendingPartialWithdrawalsWei =
      this.validatorDataClient.getTotalPendingPartialWithdrawalsWei(sortedValidatorList);
    this.metricsUpdater.setLastTotalPendingPartialWithdrawalsGwei(weiToGweiNumber(totalPendingPartialWithdrawalsWei));
    const remainingWithdrawalAmountWei = safeSub(amountWei, totalPendingPartialWithdrawalsWei);
    if (remainingWithdrawalAmountWei === 0n) {
      this.logger.info(
        `submitWithdrawalRequestsToFulfilAmount - no remaining withdrawal amount needed, amountWei=${amountWei.toString()}, totalPendingPartialWithdrawalsWei=${totalPendingPartialWithdrawalsWei.toString()}`,
      );
      return;
    }
    await this._submitPartialWithdrawalRequests(sortedValidatorList, remainingWithdrawalAmountWei);
  }

  /**
   * Submits the maximum available withdrawal requests.
   * First submits partial withdrawal requests for validators with withdrawable amounts,
   * then submits validator exit requests for validators with no withdrawable amount remaining.
   *
   * @returns {Promise<void>} A promise that resolves when all withdrawal requests are submitted (or silently returns if validator list is unavailable).
   */
  async submitMaxAvailableWithdrawalRequests(): Promise<void> {
    this.logger.info(`submitMaxAvailableWithdrawalRequests started`);
    const sortedValidatorList = await this.validatorDataClient.getValidatorsForWithdrawalRequestsAscending();
    if (sortedValidatorList === undefined) {
      this.logger.error(
        "submitMaxAvailableWithdrawalRequests failed to get sortedValidatorList with pending withdrawals",
      );
      return;
    }
    const remainingWithdrawals = await this._submitPartialWithdrawalRequests(sortedValidatorList, maxUint256);
    await this._submitValidatorExits(sortedValidatorList, remainingWithdrawals);
  }

  /**
   * Submits partial withdrawal requests for validators up to the specified amount or transaction limit.
   * Returns the number of withdrawal requests remaining (remaining shots) after this submission.
   * Processes validators in order, withdrawing up to their withdrawable amount until the target amount is reached
   * or the maximum validators per transaction limit is hit. Does unstake operation and instruments metrics after transaction success.
   *
   * @param {ValidatorBalanceWithPendingWithdrawal[]} sortedValidatorList - List of validators sorted by priority with pending withdrawals.
   * @param {bigint} amountWei - The target withdrawal amount in wei (use maxUint256 for maximum available).
   * @returns {Promise<number>} The number of withdrawal requests remaining (remaining shots) after this submission.
   */
  private async _submitPartialWithdrawalRequests(
    sortedValidatorList: ValidatorBalanceWithPendingWithdrawal[],
    amountWei: bigint,
  ): Promise<number> {
    this.logger.info(
      `_submitPartialWithdrawalRequests started amountWei=${amountWei}, sortedValidatorList.length=${sortedValidatorList.length}`,
    );
    this.logger.debug(`_submitPartialWithdrawalRequests sortedValidatorList`, { sortedValidatorList });
    const withdrawalRequests: WithdrawalRequests = {
      pubkeys: [],
      amountsGwei: [],
    };
    if (sortedValidatorList.length === 0) {
      this.logger.info(
        "_submitPartialWithdrawalRequests - sortedValidatorList is empty, returning max withdrawal requests",
      );
      return this.maxValidatorWithdrawalRequestsPerTransaction;
    }
    let totalWithdrawalRequestAmountWei = 0n;

    for (const v of sortedValidatorList) {
      if (withdrawalRequests.pubkeys.length >= this.maxValidatorWithdrawalRequestsPerTransaction) break;
      if (totalWithdrawalRequestAmountWei >= amountWei) break;

      const remainingWei = amountWei - totalWithdrawalRequestAmountWei;
      const withdrawableWei = v.withdrawableAmount * ONE_GWEI;
      const amountToWithdrawWei = min(withdrawableWei, remainingWei);
      const amountToWithdrawGwei = amountToWithdrawWei / ONE_GWEI;
      const minWithdrawalThresholdGwei = this.minWithdrawalThresholdEth * ONE_GWEI;

      if (amountToWithdrawGwei > minWithdrawalThresholdGwei) {
        withdrawalRequests.pubkeys.push(v.publicKey as Hex);
        withdrawalRequests.amountsGwei.push(amountToWithdrawGwei);
        totalWithdrawalRequestAmountWei += amountToWithdrawWei;
      }
    }

    // Do unstake
    if (totalWithdrawalRequestAmountWei === 0n || withdrawalRequests.amountsGwei.length === 0) {
      this.logger.info(
        `_submitPartialWithdrawalRequests - no withdrawal requests to submit, totalWithdrawalRequestAmountWei=${totalWithdrawalRequestAmountWei.toString()}, amountsGwei.length=${withdrawalRequests.amountsGwei.length}, returning max withdrawal requests`,
      );
      return this.maxValidatorWithdrawalRequestsPerTransaction;
    }
    await this.yieldManagerContractClient.unstake(this.yieldProvider, withdrawalRequests);

    // Instrument metrics after tx success
    for (let i = 0; i < withdrawalRequests.pubkeys.length; i++) {
      const pubkey = withdrawalRequests.pubkeys[i];
      const amountGwei = withdrawalRequests.amountsGwei[i];
      this.metricsUpdater.addValidatorPartialUnstakeAmount(pubkey, Number(amountGwei));
    }

    // Return # of remaining shots (withdrawal requests remaining)
    const remainingWithdrawals = this.maxValidatorWithdrawalRequestsPerTransaction - withdrawalRequests.pubkeys.length;
    this.logger.info(`_submitPartialWithdrawalRequests remainingWithdrawal=${remainingWithdrawals}`);
    return remainingWithdrawals;
  }

  /**
   * Submits validator exit requests for validators with no withdrawable amount remaining.
   * Processes validators that have 0 withdrawable amount, submitting them for exit.
   * Uses empty amountsGwei array to signal full withdrawals (validator exits) per contract logic:
   * - If amountsGwei.length == 0: triggers full withdrawals via TriggerableWithdrawals.addFullWithdrawalRequests
   * - If amountsGwei.length > 0: triggers amount-driven withdrawals via TriggerableWithdrawals.addWithdrawalRequests
   * Respects the remaining withdrawal slots available. Does unstake operation and instruments metrics after transaction success.
   *
   * @param {ValidatorBalanceWithPendingWithdrawal[]} sortedValidatorList - List of validators sorted by priority with pending withdrawals.
   * @param {number} remainingWithdrawals - The number of remaining withdrawal request slots available.
   * @returns {Promise<void>} A promise that resolves when validator exit requests are submitted (or silently returns if no slots available or no validators to exit).
   */
  private async _submitValidatorExits(
    sortedValidatorList: ValidatorBalanceWithPendingWithdrawal[],
    remainingWithdrawals: number,
  ): Promise<void> {
    this.logger.info(
      `_submitValidatorExits started remainingWithdrawals=${remainingWithdrawals}, sortedValidatorList.length=${sortedValidatorList.length}`,
    );
    this.logger.debug(`_submitValidatorExits sortedValidatorList`, { sortedValidatorList });
    if (remainingWithdrawals === 0 || sortedValidatorList.length === 0) {
      this.logger.info("_submitValidatorExits - no remaining withdrawals or empty validator list, skipping", {
        remainingWithdrawals,
        validatorListLength: sortedValidatorList.length,
      });
      return;
    }
    const withdrawalRequests: WithdrawalRequests = {
      pubkeys: [],
      amountsGwei: [],
    };

    for (const v of sortedValidatorList) {
      if (withdrawalRequests.pubkeys.length >= remainingWithdrawals) {
        this.logger.info(
          `_submitValidatorExits - reached remainingWithdrawals limit, breaking loop. withdrawalRequests.pubkeys.length=${withdrawalRequests.pubkeys.length}, remainingWithdrawals=${remainingWithdrawals}`,
        );
        break;
      }
      if (withdrawalRequests.pubkeys.length >= this.maxValidatorWithdrawalRequestsPerTransaction) {
        this.logger.info(
          `_submitValidatorExits - reached maxValidatorWithdrawalRequestsPerTransaction limit, breaking loop. withdrawalRequests.pubkeys.length=${withdrawalRequests.pubkeys.length}, maxValidatorWithdrawalRequestsPerTransaction=${this.maxValidatorWithdrawalRequestsPerTransaction}`,
        );
        break;
      }
      // Exit requests are ignored for validator with pending partial withdrawal - https://github.com/ethereum/consensus-specs/blob/14e6f0c919d36ef2ae7c337fe51161952a634478/specs/electra/beacon-chain.md?plain=1#L1697
      if (v.pendingWithdrawalAmount > 0n) {
        this.logger.info(
          `_submitValidatorExits - skipping validator with pending withdrawal, continuing loop. pubkey=${v.publicKey}, pendingWithdrawalAmount=${v.pendingWithdrawalAmount.toString()}`,
        );
        continue;
      }

      if (v.effectiveBalance === MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE) {
        withdrawalRequests.pubkeys.push(v.publicKey as Hex);
        // Empty amountsGwei array signals full withdrawals (validator exits) per contract logic
      }
    }

    if (withdrawalRequests.pubkeys.length === 0) {
      this.logger.info("_submitValidatorExits - no validators to exit, skipping unstake");
      return;
    }
    // Do unstake
    await this.yieldManagerContractClient.unstake(this.yieldProvider, withdrawalRequests);

    // Instrument metrics after tx success
    for (let i = 0; i < withdrawalRequests.pubkeys.length; i++) {
      const pubkey = withdrawalRequests.pubkeys[i];
      this.metricsUpdater.incrementValidatorExit(pubkey);
    }
  }
}
