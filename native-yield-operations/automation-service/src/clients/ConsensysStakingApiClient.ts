import { ApolloClient } from "@apollo/client";
import {
  IBeaconNodeAPIClient,
  ILogger,
  IRetryService,
  MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE,
  ONE_GWEI,
  PendingPartialWithdrawal,
  safeSub,
  SHARD_COMMITTEE_PERIOD,
} from "@consensys/linea-shared-utils";
import { IValidatorDataClient } from "../core/clients/IValidatorDataClient.js";
import { ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY } from "../core/entities/graphql/ActiveValidatorsByLargestBalance.js";
import { EXITED_VALIDATORS_QUERY } from "../core/entities/graphql/ExitedValidator.js";
import { EXITING_VALIDATORS_QUERY } from "../core/entities/graphql/ExitingValidators.js";
import {
  AggregatedPendingWithdrawal,
  ExitedValidator,
  ExitingValidator,
  ValidatorBalance,
  ValidatorBalanceWithPendingWithdrawal,
} from "../core/entities/Validator.js";

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
      this.apolloClient.query({ query: ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY, fetchPolicy: "network-only" }),
    );
    if (error) {
      this.logger.error("getActiveValidators error:", { error });
      return undefined;
    }
    if (!data) {
      this.logger.error("getActiveValidators data undefined");
      return undefined;
    }
    const resp = data?.allHeadValidators.nodes;
    if (!resp) {
      this.logger.info(`getActiveValidators succeeded, validatorCount=0`);
      this.logger.debug("getActiveValidators resp", { resp: undefined });
      return undefined;
    }
    // Convert GraphQL string responses to bigint
    // GraphQL returns numeric values as strings, so we need to convert them
    // BigInt() handles strings, numbers, and bigint values, so no type check needed
    type GraphQLValidatorResponse = {
      balance: string | bigint;
      effectiveBalance: string | bigint;
      publicKey: string;
      validatorIndex: string | bigint;
      activationEpoch: string | number;
    };
    const validators: ValidatorBalance[] = resp.map((v: GraphQLValidatorResponse) => ({
      balance: BigInt(v.balance),
      effectiveBalance: BigInt(v.effectiveBalance),
      publicKey: v.publicKey,
      validatorIndex: BigInt(v.validatorIndex),
      activationEpoch: Number(v.activationEpoch),
    }));
    this.logger.info(`getActiveValidators succeeded, validatorCount=${validators.length}`);
    this.logger.debug("getActiveValidators resp", { resp: validators });
    return validators;
  }

  /**
   * Retrieves all exiting validators from the Consensys Staking GraphQL API.
   * Uses retry logic to handle transient failures.
   *
   * @returns {Promise<ExitingValidator[] | undefined>} Array of exiting validators, or undefined if the query fails or returns no data.
   */
  async getExitingValidators(): Promise<ExitingValidator[] | undefined> {
    const { data, error } = await this.retryService.retry(() =>
      this.apolloClient.query({ query: EXITING_VALIDATORS_QUERY, fetchPolicy: "network-only" }),
    );
    if (error) {
      this.logger.error("getExitingValidators error:", { error });
      return undefined;
    }
    if (!data) {
      this.logger.error("getExitingValidators data undefined");
      return undefined;
    }
    const resp = data?.allHeadValidators.nodes;
    if (!resp) {
      this.logger.info(`getExitingValidators succeeded, validatorCount=0`);
      this.logger.debug("getExitingValidators resp", { resp: undefined });
      return undefined;
    }
    // Convert GraphQL string responses to appropriate types
    // GraphQL returns numeric values as strings, so we need to convert them
    // BigInt() handles strings, numbers, and bigint values, so no type check needed
    type GraphQLExitingValidatorResponse = {
      balance: string | bigint;
      effectiveBalance: string | bigint;
      publicKey: string;
      validatorIndex: string | bigint;
      exitEpoch: string | number;
      exitDate: string;
      slashed: boolean;
    };
    const validators: ExitingValidator[] = (resp as unknown as GraphQLExitingValidatorResponse[]).map((v) => ({
      balance: BigInt(v.balance),
      effectiveBalance: BigInt(v.effectiveBalance),
      publicKey: v.publicKey,
      validatorIndex: BigInt(v.validatorIndex),
      exitEpoch: typeof v.exitEpoch === "string" ? parseInt(v.exitEpoch, 10) : v.exitEpoch,
      exitDate: new Date(v.exitDate),
      slashed: Boolean(v.slashed),
    }));
    this.logger.info(`getExitingValidators succeeded, validatorCount=${validators.length}`);
    this.logger.debug("getExitingValidators resp", { resp: validators });
    return validators;
  }

  /**
   * Retrieves all fully exited validators from the Consensys Staking GraphQL API.
   * Uses retry logic to handle transient failures.
   *
   * @returns {Promise<ExitedValidator[] | undefined>} Array of fully exited validators, or undefined if the query fails or returns no data.
   */
  async getExitedValidators(): Promise<ExitedValidator[] | undefined> {
    const { data, error } = await this.retryService.retry(() =>
      this.apolloClient.query({ query: EXITED_VALIDATORS_QUERY, fetchPolicy: "network-only" }),
    );
    if (error) {
      this.logger.error("getExitedValidators error:", { error });
      return undefined;
    }
    if (!data) {
      this.logger.error("getExitedValidators data undefined");
      return undefined;
    }
    const resp = data?.allHeadValidators.nodes;
    if (!resp) {
      this.logger.info(`getExitedValidators succeeded, validatorCount=0`);
      this.logger.debug("getExitedValidators resp", { resp: undefined });
      return undefined;
    }
    // Convert GraphQL string responses to appropriate types
    // GraphQL returns numeric values as strings, so we need to convert them
    // BigInt() handles strings, numbers, and bigint values, so no type check needed
    type GraphQLExitedValidatorResponse = {
      balance: string | bigint;
      publicKey: string;
      validatorIndex: string | bigint;
      slashed: boolean;
      withdrawableEpoch: string | number;
    };
    const validators: ExitedValidator[] = (resp as unknown as GraphQLExitedValidatorResponse[]).map((v) => ({
      balance: BigInt(v.balance),
      publicKey: v.publicKey,
      validatorIndex: BigInt(v.validatorIndex),
      slashed: Boolean(v.slashed),
      withdrawableEpoch:
        typeof v.withdrawableEpoch === "string" ? parseInt(v.withdrawableEpoch, 10) : v.withdrawableEpoch,
    }));

    // Filter out validators with balance === 0
    const filteredValidators = validators.filter((v) => v.balance > 0n);

    this.logger.info(`getExitedValidators succeeded, validatorCount=${filteredValidators.length}`);
    this.logger.debug("getExitedValidators resp", { resp: filteredValidators });
    return filteredValidators;
  }

  /**
   * Joins validators with pending withdrawal information.
   * Performs the following steps:
   * 1️⃣ Aggregate duplicate pending withdrawals by validator index
   * 2️⃣ Join with validators and compute total pending amount
   *
   * @param {ValidatorBalance[] | undefined} allValidators - Array of active validators, or undefined.
   * @param {PendingPartialWithdrawal[] | undefined} pendingWithdrawalsQueue - Array of pending partial withdrawals, or undefined.
   * @returns {ValidatorBalanceWithPendingWithdrawal[] | undefined} Array of validators with pending withdrawal data, or undefined if inputs are invalid.
   */
  joinValidatorsWithPendingWithdrawals(
    allValidators: ValidatorBalance[] | undefined,
    pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
  ): ValidatorBalanceWithPendingWithdrawal[] | undefined {
    if (allValidators === undefined || pendingWithdrawalsQueue === undefined) {
      this.logger.warn("joinValidatorsWithPendingWithdrawals - invalid inputs", {
        allValidators: allValidators === undefined,
        pendingWithdrawalsQueue: pendingWithdrawalsQueue === undefined,
      });
      return undefined;
    }

    // 1️⃣ Aggregate duplicate pending withdrawals by validator index
    const pendingByValidator = new Map<number, bigint>();
    for (const w of pendingWithdrawalsQueue) {
      const current = pendingByValidator.get(w.validator_index) ?? 0n;
      pendingByValidator.set(w.validator_index, current + w.amount);
    }

    this.logger.debug("joinValidatorsWithPendingWithdrawals - aggregated pending withdrawals", {
      uniqueValidatorIndices: pendingByValidator.size,
      pendingByValidator: Array.from(pendingByValidator.entries()).map(([index, amount]) => ({
        validator_index: index,
        totalAmount: amount.toString(),
      })),
    });

    // 2️⃣ Join with validators and compute total pending amount
    const joined = allValidators.map((v) => {
      const pendingAmount = pendingByValidator.get(Number(v.validatorIndex)) ?? 0n;

      return {
        balance: v.balance,
        effectiveBalance: v.effectiveBalance,
        publicKey: v.publicKey,
        validatorIndex: v.validatorIndex,
        activationEpoch: v.activationEpoch,
        pendingWithdrawalAmount: pendingAmount,
        // N.B We expect amounts from GraphQL API and Beacon Chain RPC URL to be in gwei units, not wei.
        withdrawableAmount: safeSub(safeSub(v.balance, pendingAmount), MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE),
      };
    });

    this.logger.debug("joinValidatorsWithPendingWithdrawals - joined results", {
      joinedCount: joined.length,
      validatorsWithPendingWithdrawals: joined.filter((v) => v.pendingWithdrawalAmount > 0n).length,
      allEntries: joined.map((v) => ({
        validatorIndex: v.validatorIndex.toString(),
        publicKey: v.publicKey,
        pendingWithdrawalAmount: v.pendingWithdrawalAmount.toString(),
        withdrawableAmount: v.withdrawableAmount.toString(),
      })),
    });

    return joined;
  }

  /**
   * Filters pending withdrawals to only include those matching active validators,
   * then aggregates by validator_index and withdrawable_epoch.
   * Performs the following steps:
   * 1️⃣ Filter pending withdrawals to match active validators
   * 2️⃣ Aggregate amounts by validator_index and withdrawable_epoch
   * 3️⃣ Include pubkey from matching validators
   *
   * @param {ValidatorBalance[] | undefined} allValidators - Array of active validators, or undefined.
   * @param {PendingPartialWithdrawal[] | undefined} pendingWithdrawalsQueue - Array of pending partial withdrawals, or undefined.
   * @returns {AggregatedPendingWithdrawal[] | undefined} Array of aggregated pending withdrawals with pubkey, or undefined if inputs are invalid.
   */
  getFilteredAndAggregatedPendingWithdrawals(
    allValidators: ValidatorBalance[] | undefined,
    pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
  ): AggregatedPendingWithdrawal[] | undefined {
    if (allValidators === undefined || pendingWithdrawalsQueue === undefined) {
      this.logger.warn("getFilteredAndAggregatedPendingWithdrawals - invalid inputs", {
        allValidators: allValidators === undefined,
        pendingWithdrawalsQueue: pendingWithdrawalsQueue === undefined,
      });
      return undefined;
    }

    // 1️⃣ Create Set of validator indices from allValidators for efficient lookup
    const validatorIndicesSet = new Set<number>();
    const pubkeyByValidatorIndex = new Map<number, string>();
    for (const v of allValidators) {
      const index = Number(v.validatorIndex);
      validatorIndicesSet.add(index);
      pubkeyByValidatorIndex.set(index, v.publicKey);
    }

    // 2️⃣ Filter pending withdrawals to only include those matching active validators
    const filteredWithdrawals = pendingWithdrawalsQueue.filter((w) => validatorIndicesSet.has(w.validator_index));

    this.logger.debug("getFilteredAndAggregatedPendingWithdrawals - filtered withdrawals", {
      totalPendingWithdrawals: pendingWithdrawalsQueue.length,
      filteredCount: filteredWithdrawals.length,
      uniqueValidatorIndices: validatorIndicesSet.size,
    });

    // 3️⃣ Aggregate amounts by validator_index and withdrawable_epoch
    const aggregatedMap = new Map<string, bigint>();
    for (const w of filteredWithdrawals) {
      const key = `${w.validator_index}-${w.withdrawable_epoch}`;
      const current = aggregatedMap.get(key) ?? 0n;
      aggregatedMap.set(key, current + w.amount);
    }

    // 4️⃣ Convert Map to array with pubkey included
    const result: AggregatedPendingWithdrawal[] = Array.from(aggregatedMap.entries()).map(([key, amount]) => {
      const [validator_index_str, withdrawable_epoch_str] = key.split("-");
      const validator_index = parseInt(validator_index_str, 10);
      const withdrawable_epoch = parseInt(withdrawable_epoch_str, 10);
      const pubkey = pubkeyByValidatorIndex.get(validator_index) ?? "";

      return {
        validator_index,
        withdrawable_epoch,
        amount,
        pubkey,
      };
    });

    this.logger.info(`getFilteredAndAggregatedPendingWithdrawals succeeded, aggregatedCount=${result.length}`);
    this.logger.debug("getFilteredAndAggregatedPendingWithdrawals - aggregated results", {
      aggregatedCount: result.length,
      allEntries: result.map((r) => ({
        validator_index: r.validator_index,
        withdrawable_epoch: r.withdrawable_epoch,
        amount: r.amount.toString(),
        pubkey: r.pubkey,
      })),
    });

    return result;
  }

  /**
   * Retrieves validators sorted by withdrawable amount for withdrawal requests.
   * Returns sorted in ascending order of withdrawableAmount (smallest first).
   * Fetches data, delegates to joinValidatorsWithPendingWithdrawals for processing, filters by shard committee period eligibility, and sorts the result.
   * Only includes validators that have been active for at least SHARD_COMMITTEE_PERIOD epochs.
   *
   * @returns {Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined>} Array of eligible validators sorted ascending by withdrawableAmount for withdrawal requests, or undefined if data retrieval fails.
   */
  async getValidatorsForWithdrawalRequestsAscending(): Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined> {
    const [allValidators, pendingWithdrawalsQueue] = await Promise.all([
      this.getActiveValidators(),
      this.beaconNodeApiClient.getPendingPartialWithdrawals(),
    ]);
    if (allValidators === undefined || pendingWithdrawalsQueue === undefined) {
      this.logger.warn(
        "getValidatorsForWithdrawalRequestsAscending - failed to retrieve validators or pending withdrawals",
        {
          allValidators: allValidators === undefined,
          pendingWithdrawalsQueue: pendingWithdrawalsQueue === undefined,
        },
      );
      return undefined;
    }

    const joined = this.joinValidatorsWithPendingWithdrawals(allValidators, pendingWithdrawalsQueue);
    if (joined === undefined) {
      this.logger.warn(
        "getValidatorsForWithdrawalRequestsAscending - joinValidatorsWithPendingWithdrawals returned undefined",
      );
      return undefined;
    }

    // Get current epoch to filter validators by shard committee period
    const currentEpoch = await this.beaconNodeApiClient.getCurrentEpoch();
    let eligibleValidators = joined;
    if (currentEpoch === undefined) {
      this.logger.warn(
        "getValidatorsForWithdrawalRequestsAscending - failed to retrieve current epoch, skipping filter",
      );
    } else {
      // Filter out validators that haven't been active for at least SHARD_COMMITTEE_PERIOD epochs
      eligibleValidators = joined.filter((v) => v.activationEpoch + SHARD_COMMITTEE_PERIOD <= currentEpoch);
    }

    // ✅ Sort ascending (smallest withdrawableAmount first)
    eligibleValidators.sort((a, b) =>
      a.withdrawableAmount < b.withdrawableAmount ? -1 : a.withdrawableAmount > b.withdrawableAmount ? 1 : 0,
    );

    this.logger.info(
      `getValidatorsForWithdrawalRequestsAscending succeeded, validatorCount=${eligibleValidators.length}`,
    );
    this.logger.debug("getValidatorsForWithdrawalRequestsAscending joined", { joined: eligibleValidators });
    return eligibleValidators;
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
    this.logger.info(`getTotalPendingPartialWithdrawalsWei totalWei=${totalWei}`);
    return totalWei;
  }

  /**
   * Calculates the total balance across all validators in gwei.
   * Aggregates the balance field from all ValidatorBalance elements.
   *
   * @param {ValidatorBalance[] | undefined} validators - Array of validators, or undefined.
   * @returns {bigint | undefined} Total validator balance in gwei, or undefined if input is undefined or empty.
   */
  getTotalValidatorBalanceGwei(validators: ValidatorBalance[] | undefined): bigint | undefined {
    if (validators === undefined || validators.length === 0) {
      return undefined;
    }
    const totalGwei = validators.reduce((acc, v) => acc + v.balance, 0n);
    this.logger.info(`getTotalValidatorBalanceGwei totalGwei=${totalGwei}`);
    return totalGwei;
  }

  /**
   * Filters exiting validators to only include those that have been slashed.
   *
   * @param {ExitingValidator[] | undefined} validators - Array of exiting validators, or undefined.
   * @returns {ExitingValidator[] | undefined} Array of slashed validators, or undefined if input is undefined.
   */
  getSlashedValidators(validators: ExitingValidator[] | undefined): ExitingValidator[] | undefined {
    if (validators === undefined) {
      this.logger.warn("getSlashedValidators - invalid input: validators is undefined");
      return undefined;
    }
    const slashedValidators = validators.filter((v) => v.slashed === true);
    this.logger.info(`getSlashedValidators succeeded, slashedCount=${slashedValidators.length}`);
    return slashedValidators;
  }

  /**
   * Filters exiting validators to only include those that have not been slashed.
   *
   * @param {ExitingValidator[] | undefined} validators - Array of exiting validators, or undefined.
   * @returns {ExitingValidator[] | undefined} Array of non-slashed exiting validators, or undefined if input is undefined.
   */
  getNonSlashedAndExitingValidators(validators: ExitingValidator[] | undefined): ExitingValidator[] | undefined {
    if (validators === undefined) {
      this.logger.warn("getNonSlashedAndExitingValidators - invalid input: validators is undefined");
      return undefined;
    }
    const nonSlashedValidators = validators.filter((v) => v.slashed === false);
    this.logger.info(`getNonSlashedAndExitingValidators succeeded, nonSlashedCount=${nonSlashedValidators.length}`);
    return nonSlashedValidators;
  }

  /**
   * Calculates the total balance across all exiting validators in gwei.
   * Aggregates the balance field from all ExitingValidator elements.
   *
   * @param {ExitingValidator[] | undefined} validators - Array of exiting validators, or undefined.
   * @returns {bigint | undefined} Total exiting validator balance in gwei. Returns undefined if input is undefined (data unavailable), returns 0n if input is empty array (no validators exist).
   */
  getTotalBalanceOfExitingValidators(validators: ExitingValidator[] | undefined): bigint | undefined {
    if (validators === undefined) {
      this.logger.warn("getTotalBalanceOfExitingValidators - invalid input: validators is undefined", {
        validators: true,
      });
      return undefined;
    }
    if (validators.length === 0) {
      this.logger.info("getTotalBalanceOfExitingValidators - empty array, returning 0");
      return 0n;
    }
    const totalGwei = validators.reduce((acc, v) => acc + v.balance, 0n);
    this.logger.info(`getTotalBalanceOfExitingValidators totalGwei=${totalGwei}`);
    return totalGwei;
  }

  /**
   * Calculates the total balance across all exited validators in gwei.
   * Aggregates the balance field from all ExitedValidator elements.
   *
   * @param {ExitedValidator[] | undefined} validators - Array of exited validators, or undefined.
   * @returns {bigint | undefined} Total exited validator balance in gwei. Returns undefined if input is undefined (data unavailable), returns 0n if input is empty array (no validators exist).
   */
  getTotalBalanceOfExitedValidators(validators: ExitedValidator[] | undefined): bigint | undefined {
    if (validators === undefined) {
      this.logger.warn("getTotalBalanceOfExitedValidators - invalid input: validators is undefined", {
        validators: true,
      });
      return undefined;
    }
    if (validators.length === 0) {
      this.logger.info("getTotalBalanceOfExitedValidators - empty array, returning 0");
      return 0n;
    }
    const totalGwei = validators.reduce((acc, v) => acc + v.balance, 0n);
    this.logger.info(`getTotalBalanceOfExitedValidators totalGwei=${totalGwei}`);
    return totalGwei;
  }
}
