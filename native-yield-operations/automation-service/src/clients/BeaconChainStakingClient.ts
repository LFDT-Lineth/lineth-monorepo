import { ILogger, min, ONE_GWEI, safeSub } from "@consensys/linea-shared-utils";
import { IBeaconChainStakingClient } from "../core/clients/IBeaconChainStakingClient.js";
import { IValidatorDataClient } from "../core/clients/IValidatorDataClient.js";
import { ValidatorBalanceWithPendingWithdrawal } from "../core/entities/ValidatorBalance.js";
import { WithdrawalRequests } from "../core/entities/LidoStakingVaultWithdrawalParams.js";
import { Address, maxUint256, stringToHex, TransactionReceipt } from "viem";
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
   */
  constructor(
    private readonly logger: ILogger,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly validatorDataClient: IValidatorDataClient,
    private readonly maxValidatorWithdrawalRequestsPerTransaction: number,
    private readonly yieldManagerContractClient: IYieldManager<TransactionReceipt>,
    private readonly yieldProvider: Address,
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
    this.logger.debug(
      `submitWithdrawalRequestsToFulfilAmount started: amountWei=${amountWei.toString()}; validatorLimit=${this.maxValidatorWithdrawalRequestsPerTransaction}`,
    );
    const sortedValidatorList = await this.validatorDataClient.getActiveValidatorsWithPendingWithdrawals();
    if (sortedValidatorList === undefined) {
      this.logger.error(
        "submitWithdrawalRequestsToFulfilAmount failed to get sortedValidatorList with pending withdrawals",
      );
      return;
    }
    const totalPendingPartialWithdrawalsWei =
      this.validatorDataClient.getTotalPendingPartialWithdrawalsWei(sortedValidatorList);
    const remainingWithdrawalAmountWei = safeSub(amountWei, totalPendingPartialWithdrawalsWei);
    if (remainingWithdrawalAmountWei === 0n) return;
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
    this.logger.debug(`submitMaxAvailableWithdrawalRequests started`);
    const sortedValidatorList = await this.validatorDataClient.getActiveValidatorsWithPendingWithdrawals();
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
    this.logger.debug(`_submitPartialWithdrawalRequests started amountWei=${amountWei}`, { sortedValidatorList });
    const withdrawalRequests: WithdrawalRequests = {
      pubkeys: [],
      amountsGwei: [],
    };
    if (sortedValidatorList.length === 0) return this.maxValidatorWithdrawalRequestsPerTransaction;
    let totalWithdrawalRequestAmountWei = 0n;

    for (const v of sortedValidatorList) {
      if (withdrawalRequests.pubkeys.length >= this.maxValidatorWithdrawalRequestsPerTransaction) break;
      if (totalWithdrawalRequestAmountWei >= amountWei) break;

      const remainingWei = amountWei - totalWithdrawalRequestAmountWei;
      const withdrawableWei = v.withdrawableAmount * ONE_GWEI;
      const amountToWithdrawWei = min(withdrawableWei, remainingWei);
      const amountToWithdrawGwei = amountToWithdrawWei / ONE_GWEI;

      if (amountToWithdrawGwei > 0n) {
        withdrawalRequests.pubkeys.push(stringToHex(v.publicKey));
        withdrawalRequests.amountsGwei.push(amountToWithdrawGwei);
        totalWithdrawalRequestAmountWei += amountToWithdrawWei;
      }
    }

    // Do unstake
    if (totalWithdrawalRequestAmountWei === 0n || withdrawalRequests.amountsGwei.length === 0) {
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
    this.logger.debug(`_submitPartialWithdrawalRequests remainingWithdrawal=${remainingWithdrawals}`);
    return remainingWithdrawals;
  }

  /**
   * Submits validator exit requests for validators with no withdrawable amount remaining.
   * Processes validators that have 0 withdrawable amount, submitting them for exit using 0 amount as a signal for validator exit.
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
    this.logger.debug(`_submitValidatorExits started remainingWithdrawals=${remainingWithdrawals}`, {
      sortedValidatorList,
    });
    if (remainingWithdrawals === 0 || sortedValidatorList.length === 0) return;
    const withdrawalRequests: WithdrawalRequests = {
      pubkeys: [],
      amountsGwei: [],
    };

    for (const v of sortedValidatorList) {
      if (withdrawalRequests.pubkeys.length >= remainingWithdrawals) break;
      if (withdrawalRequests.pubkeys.length >= this.maxValidatorWithdrawalRequestsPerTransaction) break;

      if (v.withdrawableAmount === 0n) {
        withdrawalRequests.pubkeys.push(stringToHex(v.publicKey));
        // 0 amount -> signal for validator exit
        withdrawalRequests.amountsGwei.push(0n);
      }
    }

    if (withdrawalRequests.amountsGwei.length === 0) return;
    // Do unstake
    await this.yieldManagerContractClient.unstake(this.yieldProvider, withdrawalRequests);

    // Instrument metrics after tx success
    for (let i = 0; i < withdrawalRequests.pubkeys.length; i++) {
      const pubkey = withdrawalRequests.pubkeys[i];
      this.metricsUpdater.incrementValidatorExit(pubkey);
    }
  }
}
