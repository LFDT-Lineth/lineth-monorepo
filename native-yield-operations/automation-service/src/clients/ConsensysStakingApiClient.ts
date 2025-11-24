import { ApolloClient } from "@apollo/client";
import { IBeaconNodeAPIClient, ILogger, IRetryService, ONE_GWEI, safeSub } from "@consensys/linea-shared-utils";
import { IValidatorDataClient } from "../core/clients/IValidatorDataClient.js";
import { ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY } from "../core/entities/graphql/ActiveValidatorsByLargestBalance.js";
import { ValidatorBalance, ValidatorBalanceWithPendingWithdrawal } from "../core/entities/ValidatorBalance.js";

/**
 * Client for retrieving validator data from Consensys Staking API via GraphQL.
 * Fetches active validators and combines them with pending withdrawal data from the beacon chain.
 */
export class ConsensysStakingApiClient implements IValidatorDataClient {
  /**
   * Creates a new ConsensysStakingApiClient instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {IRetryService} retryService - Service for retrying failed operations.
   * @param {ApolloClient} apolloClient - Apollo GraphQL client for querying Consensys Staking API.
   * @param {IBeaconNodeAPIClient} beaconNodeApiClient - Client for retrieving pending partial withdrawals from beacon chain.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly retryService: IRetryService,
    private readonly apolloClient: ApolloClient,
    private readonly beaconNodeApiClient: IBeaconNodeAPIClient,
  ) {}

  /**
   * Retrieves all active validators from the Consensys Staking GraphQL API.
   * Uses retry logic to handle transient failures.
   *
   * @returns {Promise<ValidatorBalance[] | undefined>} Array of active validators, or undefined if the query fails or returns no data.
   */
  async getActiveValidators(): Promise<ValidatorBalance[] | undefined> {
    const { data, error } = await this.retryService.retry(() =>
      this.apolloClient.query({ query: ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY }),
    );
    if (error) {
      this.logger.error("getActiveValidators error:", { error });
      return undefined;
    }
    if (!data) {
      this.logger.error("getActiveValidators data undefined");
      return undefined;
    }
    const resp = data?.allValidators.nodes;
    this.logger.debug("getActiveValidators succeded", { resp });
    return resp;
  }

  /**
   * Retrieves active validators with pending withdrawal information.
   * Returns sorted in descending order of withdrawableValue (largest withdrawableAmount first).
   * Performs the following steps:
   * 1️⃣ Aggregate duplicate pending withdrawals by validator index
   * 2️⃣ Join with validators and compute total pending amount
   * ✅ Sort descending (largest withdrawableAmount first)
   *
   * @returns {Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined>} Array of validators with pending withdrawal data, sorted descending by withdrawableAmount, or undefined if data retrieval fails.
   */
  async getActiveValidatorsWithPendingWithdrawals(): Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined> {
    const [allValidators, pendingWithdrawalsQueue] = await Promise.all([
      this.getActiveValidators(),
      this.beaconNodeApiClient.getPendingPartialWithdrawals(),
    ]);
    if (allValidators === undefined || pendingWithdrawalsQueue === undefined) return undefined;

    // 1️⃣ Aggregate duplicate pending withdrawals by validator index
    const pendingByValidator = new Map<number, bigint>();
    for (const w of pendingWithdrawalsQueue) {
      const current = pendingByValidator.get(w.validator_index) ?? 0n;
      pendingByValidator.set(w.validator_index, current + w.amount);
    }

    // 2️⃣ Join with validators and compute total pending amount
    const joined = allValidators.map((v) => {
      const pendingAmount = pendingByValidator.get(Number(v.validatorIndex)) ?? 0n;

      return {
        balance: v.balance,
        effectiveBalance: v.effectiveBalance,
        publicKey: v.publicKey,
        validatorIndex: v.validatorIndex,
        pendingWithdrawalAmount: pendingAmount,
        // N.B We expect amounts from GraphQL API and Beacon Chain RPC URL to be in gwei units, not wei.
        withdrawableAmount: safeSub(safeSub(v.balance, pendingAmount), ONE_GWEI * 32n),
      };
    });

    // ✅ Sort descending (largest withdrawableAmount first)
    joined.sort((a, b) =>
      a.withdrawableAmount > b.withdrawableAmount ? -1 : a.withdrawableAmount < b.withdrawableAmount ? 1 : 0,
    );

    this.logger.debug("getActiveValidatorsWithPendingWithdrawals return val", { joined });
    return joined;
  }

  /**
   * Calculates the total pending partial withdrawals across all validators in wei.
   * Should be static, but bit tricky to use static and interface together.
   *
   * @param {ValidatorBalanceWithPendingWithdrawal[]} validatorList - List of validators with pending withdrawal information.
   * @returns {bigint} Total pending partial withdrawals amount in wei.
   */
  getTotalPendingPartialWithdrawalsWei(validatorList: ValidatorBalanceWithPendingWithdrawal[]): bigint {
    const totalGwei = validatorList.reduce((acc, v) => acc + v.pendingWithdrawalAmount, 0n);
    const totalWei = totalGwei * ONE_GWEI;
    this.logger.debug(`getTotalPendingPartialWithdrawalsWei totalWei=${totalWei}`);
    return totalWei;
  }
}
