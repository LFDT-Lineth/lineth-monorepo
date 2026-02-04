import { jest } from "@jest/globals";
import type { ApolloClient } from "@apollo/client";
import {
  IBeaconNodeAPIClient,
  IRetryService,
  ONE_GWEI,
  PendingPartialWithdrawal,
  safeSub,
  SHARD_COMMITTEE_PERIOD,
} from "@consensys/linea-shared-utils";

import { createLoggerMock } from "../../__tests__/helpers/index.js";
import { ConsensysStakingApiClient } from "../ConsensysStakingApiClient.js";
import { ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY } from "../../core/entities/graphql/ActiveValidatorsByLargestBalance.js";
import { EXITING_VALIDATORS_QUERY } from "../../core/entities/graphql/ExitingValidators.js";
import type {
  ExitedValidator,
  ExitingValidator,
  ValidatorBalance,
  ValidatorBalanceWithPendingWithdrawal,
} from "../../core/entities/Validator.js";

// Semantic constants
const VALIDATOR_32_ETH = 32n * ONE_GWEI;
const VALIDATOR_34_ETH = 34n * ONE_GWEI;
const VALIDATOR_35_ETH = 35n * ONE_GWEI;
const VALIDATOR_40_ETH = 40n * ONE_GWEI;
const VALIDATOR_45_ETH = 45n * ONE_GWEI;
const WITHDRAWAL_1_ETH = 1n * ONE_GWEI;
const WITHDRAWAL_2_ETH = 2n * ONE_GWEI;
const WITHDRAWAL_3_ETH = 3n * ONE_GWEI;
const WITHDRAWAL_4_ETH = 4n * ONE_GWEI;
const WITHDRAWAL_5_ETH = 5n * ONE_GWEI;
const WITHDRAWAL_6_ETH = 6n * ONE_GWEI;
const EPOCH_0 = 0;
const EPOCH_100 = 100;
const EPOCH_200 = 200;
const VALIDATOR_INDEX_1 = 1n;
const VALIDATOR_INDEX_2 = 2n;
const VALIDATOR_INDEX_10 = 10n;
const EXIT_DATE_STRING = "2024-01-15T10:30:00Z";

const createRetryService = (): jest.Mocked<IRetryService> => {
  const retryMock = jest.fn(async (fn: () => Promise<unknown>, _timeoutMs?: number) => fn());
  return { retry: retryMock } as unknown as jest.Mocked<IRetryService>;
};

const createApolloClient = (): ApolloClient & { query: jest.MockedFunction<() => Promise<{ data?: unknown; error?: unknown }>> } => {
  return {
    query: jest.fn<() => Promise<{ data?: unknown; error?: unknown }>>(),
  } as unknown as ApolloClient & { query: jest.MockedFunction<() => Promise<{ data?: unknown; error?: unknown }>> };
};

const createBeaconNodeApiClient = (): jest.Mocked<IBeaconNodeAPIClient> => {
  const pendingWithdrawalsMock = jest.fn() as jest.MockedFunction<
    IBeaconNodeAPIClient["getPendingPartialWithdrawals"]
  >;
  const getCurrentEpochMock = jest.fn() as jest.MockedFunction<IBeaconNodeAPIClient["getCurrentEpoch"]>;
  return {
    getPendingPartialWithdrawals: pendingWithdrawalsMock,
    getCurrentEpoch: getCurrentEpochMock,
  } as unknown as jest.Mocked<IBeaconNodeAPIClient>;
};

const createValidatorBalance = (overrides: Partial<ValidatorBalance> = {}): ValidatorBalance => ({
  balance: VALIDATOR_32_ETH,
  effectiveBalance: VALIDATOR_32_ETH,
  publicKey: "validator-1",
  validatorIndex: VALIDATOR_INDEX_1,
  activationEpoch: EPOCH_0,
  ...overrides,
});

const createExitingValidator = (overrides: Partial<ExitingValidator> = {}): ExitingValidator => ({
  balance: VALIDATOR_32_ETH,
  effectiveBalance: VALIDATOR_32_ETH,
  publicKey: "validator-1",
  validatorIndex: VALIDATOR_INDEX_1,
  exitEpoch: EPOCH_100,
  exitDate: new Date(EXIT_DATE_STRING),
  slashed: false,
  ...overrides,
});

const createExitedValidator = (overrides: Partial<ExitedValidator> = {}): ExitedValidator => ({
  balance: VALIDATOR_32_ETH,
  publicKey: "validator-1",
  validatorIndex: VALIDATOR_INDEX_1,
  slashed: false,
  withdrawableEpoch: EPOCH_100,
  ...overrides,
});

const createGraphQLActiveValidatorResponse = (overrides = {}) => ({
  balance: "32000000000",
  effectiveBalance: "32000000000",
  publicKey: "validator-1",
  validatorIndex: "1",
  activationEpoch: "0",
  ...overrides,
});

const createGraphQLExitingValidatorResponse = (overrides = {}) => ({
  balance: "32000000000",
  effectiveBalance: "32000000000",
  publicKey: "validator-1",
  validatorIndex: "1",
  exitEpoch: "100",
  exitDate: EXIT_DATE_STRING,
  slashed: false,
  ...overrides,
});

const createGraphQLExitedValidatorResponse = (overrides = {}) => ({
  balance: "32000000000",
  publicKey: "validator-1",
  validatorIndex: "1",
  slashed: false,
  withdrawableEpoch: "100",
  ...overrides,
});

const createPendingWithdrawal = (overrides: Partial<PendingPartialWithdrawal> = {}): PendingPartialWithdrawal => ({
  validator_index: 1,
  amount: WITHDRAWAL_2_ETH,
  withdrawable_epoch: EPOCH_0,
  ...overrides,
});

describe("ConsensysStakingApiClient", () => {
  describe("getActiveValidators", () => {
    it("returns undefined and logs error when query returns error", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const queryError = new Error("graphql failure");
      (retryService.retry as jest.MockedFunction<typeof retryService.retry>).mockImplementationOnce(
        async () => ({ data: undefined, error: queryError }) as never,
      );

      // Act
      const result = await client.getActiveValidators();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("getActiveValidators error:", { error: queryError });
    });

    it("returns undefined and logs error when query response lacks data", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      (retryService.retry as jest.MockedFunction<typeof retryService.retry>).mockImplementationOnce(
        async () => ({ data: undefined, error: undefined }) as never,
      );

      // Act
      const result = await client.getActiveValidators();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("getActiveValidators data undefined");
    });

    it("returns undefined and logs success when nodes array is undefined", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      apolloClient.query.mockResolvedValue({
        data: { allHeadValidators: { nodes: undefined } },
        error: undefined,
      });

      // Act
      const result = await client.getActiveValidators();

      // Assert
      expect(result).toBeUndefined();
      expect(apolloClient.query).toHaveBeenCalledWith({
        query: ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY,
        fetchPolicy: "network-only",
      });
      expect(logger.info).toHaveBeenCalledWith("getActiveValidators succeeded, validatorCount=0");
      expect(logger.debug).toHaveBeenCalledWith("getActiveValidators resp", { resp: undefined });
    });

    it("returns validator list and logs success when query succeeds", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const graphqlResponse = [createGraphQLActiveValidatorResponse()];
      const expectedValidator = createValidatorBalance();
      apolloClient.query.mockResolvedValue({
        data: { allHeadValidators: { nodes: graphqlResponse } },
        error: undefined,
      });

      // Act
      const result = await client.getActiveValidators();

      // Assert
      expect(result).toEqual([expectedValidator]);
      expect(apolloClient.query).toHaveBeenCalledWith({
        query: ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY,
        fetchPolicy: "network-only",
      });
      expect(logger.info).toHaveBeenCalledWith("getActiveValidators succeeded, validatorCount=1");
      expect(logger.debug).toHaveBeenCalledWith("getActiveValidators resp", { resp: [expectedValidator] });
    });
  });

  describe("getExitingValidators", () => {
    it("returns undefined and logs error when query returns error", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const queryError = new Error("graphql failure");
      (retryService.retry as jest.MockedFunction<typeof retryService.retry>).mockImplementationOnce(
        async () => ({ data: undefined, error: queryError }) as never,
      );

      // Act
      const result = await client.getExitingValidators();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("getExitingValidators error:", { error: queryError });
    });

    it("returns undefined and logs error when query response lacks data", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      (retryService.retry as jest.MockedFunction<typeof retryService.retry>).mockImplementationOnce(
        async () => ({ data: undefined, error: undefined }) as never,
      );

      // Act
      const result = await client.getExitingValidators();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("getExitingValidators data undefined");
    });

    it("returns validator list with converted types when query succeeds", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const graphqlResponse = [createGraphQLExitingValidatorResponse()];
      const expectedValidator = createExitingValidator();
      apolloClient.query.mockResolvedValue({
        data: { allHeadValidators: { nodes: graphqlResponse } },
        error: undefined,
      });

      // Act
      const result = await client.getExitingValidators();

      // Assert
      expect(result).toEqual([expectedValidator]);
      expect(apolloClient.query).toHaveBeenCalledWith({
        query: EXITING_VALIDATORS_QUERY,
        fetchPolicy: "network-only",
      });
      expect(logger.info).toHaveBeenCalledWith("getExitingValidators succeeded, validatorCount=1");
    });

    it("converts exitEpoch from string to number", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const graphqlResponse = [createGraphQLExitingValidatorResponse({ exitEpoch: "200" })];
      apolloClient.query.mockResolvedValue({
        data: { allHeadValidators: { nodes: graphqlResponse } },
        error: undefined,
      });

      // Act
      const result = await client.getExitingValidators();

      // Assert
      expect(result).toBeDefined();
      expect(result![0].exitEpoch).toBe(200);
    });

    it("preserves exitEpoch when already a number", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const graphqlResponse = [createGraphQLExitingValidatorResponse({ exitEpoch: 200 })];
      apolloClient.query.mockResolvedValue({
        data: { allHeadValidators: { nodes: graphqlResponse } },
        error: undefined,
      });

      // Act
      const result = await client.getExitingValidators();

      // Assert
      expect(result).toBeDefined();
      expect(result![0].exitEpoch).toBe(200);
    });
  });

  describe("getExitedValidators", () => {
    it("returns undefined and logs error when query returns error", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const queryError = new Error("graphql failure");
      (retryService.retry as jest.MockedFunction<typeof retryService.retry>).mockImplementationOnce(
        async () => ({ data: undefined, error: queryError }) as never,
      );

      // Act
      const result = await client.getExitedValidators();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("getExitedValidators error:", { error: queryError });
    });

    it("returns validator list excluding zero balance validators", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const graphqlResponse = [
        createGraphQLExitedValidatorResponse({ balance: "0", validatorIndex: "10", publicKey: "validator-zero" }),
        createGraphQLExitedValidatorResponse({ balance: "35000000000", validatorIndex: "20", publicKey: "validator-nonzero" }),
      ];
      apolloClient.query.mockResolvedValue({
        data: { allHeadValidators: { nodes: graphqlResponse } },
        error: undefined,
      });

      // Act
      const result = await client.getExitedValidators();

      // Assert
      expect(result).toHaveLength(1);
      expect(result![0].balance).toBe(VALIDATOR_35_ETH);
      expect(result![0].publicKey).toBe("validator-nonzero");
      expect(logger.info).toHaveBeenCalledWith("getExitedValidators succeeded, validatorCount=1");
    });

    it("converts withdrawableEpoch from string to number", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const graphqlResponse = [createGraphQLExitedValidatorResponse({ withdrawableEpoch: "200" })];
      apolloClient.query.mockResolvedValue({
        data: { allHeadValidators: { nodes: graphqlResponse } },
        error: undefined,
      });

      // Act
      const result = await client.getExitedValidators();

      // Assert
      expect(result).toBeDefined();
      expect(result![0].withdrawableEpoch).toBe(200);
    });
  });

  describe("joinValidatorsWithPendingWithdrawals", () => {
    it("returns undefined and logs warning when validators are undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const pendingWithdrawals = [createPendingWithdrawal()];

      // Act
      const result = client.joinValidatorsWithPendingWithdrawals(undefined, pendingWithdrawals);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("joinValidatorsWithPendingWithdrawals - invalid inputs", {
        allValidators: true,
        pendingWithdrawalsQueue: false,
      });
    });

    it("returns undefined and logs warning when pending withdrawals are undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators = [createValidatorBalance()];

      // Act
      const result = client.joinValidatorsWithPendingWithdrawals(validators, undefined);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("joinValidatorsWithPendingWithdrawals - invalid inputs", {
        allValidators: false,
        pendingWithdrawalsQueue: true,
      });
    });

    it("aggregates pending withdrawals by validator index and computes withdrawable amounts", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validatorA = createValidatorBalance({
        balance: VALIDATOR_40_ETH,
        validatorIndex: VALIDATOR_INDEX_1,
        publicKey: "validator-a",
      });
      const validatorB = createValidatorBalance({
        balance: VALIDATOR_34_ETH,
        validatorIndex: VALIDATOR_INDEX_2,
        publicKey: "validator-b",
      });
      const pendingWithdrawals = [
        createPendingWithdrawal({ validator_index: 1, amount: WITHDRAWAL_2_ETH, withdrawable_epoch: EPOCH_0 }),
        createPendingWithdrawal({ validator_index: 1, amount: WITHDRAWAL_3_ETH, withdrawable_epoch: 1 }),
        createPendingWithdrawal({ validator_index: 2, amount: WITHDRAWAL_1_ETH, withdrawable_epoch: EPOCH_0 }),
      ];

      // Act
      const result = client.joinValidatorsWithPendingWithdrawals([validatorB, validatorA], pendingWithdrawals);

      // Assert
      const expectedValidatorA: ValidatorBalanceWithPendingWithdrawal = {
        ...validatorA,
        pendingWithdrawalAmount: WITHDRAWAL_5_ETH,
        withdrawableAmount: safeSub(safeSub(validatorA.balance, WITHDRAWAL_5_ETH), VALIDATOR_32_ETH),
      };
      const expectedValidatorB: ValidatorBalanceWithPendingWithdrawal = {
        ...validatorB,
        pendingWithdrawalAmount: WITHDRAWAL_1_ETH,
        withdrawableAmount: safeSub(safeSub(validatorB.balance, WITHDRAWAL_1_ETH), VALIDATOR_32_ETH),
      };
      expect(result).toEqual([expectedValidatorB, expectedValidatorA]);
      expect(logger.debug).toHaveBeenCalledWith("joinValidatorsWithPendingWithdrawals - aggregated pending withdrawals", {
        uniqueValidatorIndices: 2,
        pendingByValidator: expect.arrayContaining([
          { validator_index: 1, totalAmount: WITHDRAWAL_5_ETH.toString() },
          { validator_index: 2, totalAmount: WITHDRAWAL_1_ETH.toString() },
        ]),
      });
    });

    it("handles validators with no pending withdrawals", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validator = createValidatorBalance();

      // Act
      const result = client.joinValidatorsWithPendingWithdrawals([validator], []);

      // Assert
      const expected: ValidatorBalanceWithPendingWithdrawal = {
        ...validator,
        pendingWithdrawalAmount: 0n,
        withdrawableAmount: safeSub(safeSub(validator.balance, 0n), VALIDATOR_32_ETH),
      };
      expect(result).toEqual([expected]);
      expect(logger.debug).toHaveBeenCalledWith("joinValidatorsWithPendingWithdrawals - aggregated pending withdrawals", {
        uniqueValidatorIndices: 0,
        pendingByValidator: [],
      });
    });

    it("returns empty array when validators array is empty", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const pendingWithdrawals = [createPendingWithdrawal()];

      // Act
      const result = client.joinValidatorsWithPendingWithdrawals([], pendingWithdrawals);

      // Assert
      expect(result).toEqual([]);
    });
  });

  describe("getValidatorsForWithdrawalRequestsAscending", () => {
    it("returns undefined when active validator data is unavailable", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      jest.spyOn(client, "getActiveValidators").mockResolvedValueOnce(undefined);
      beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValueOnce([]);

      // Act
      const result = await client.getValidatorsForWithdrawalRequestsAscending();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith(
        "getValidatorsForWithdrawalRequestsAscending - failed to retrieve validators or pending withdrawals",
        { allValidators: true, pendingWithdrawalsQueue: false },
      );
    });

    it("returns undefined when pending withdrawals cannot be fetched", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators = [createValidatorBalance()];
      jest.spyOn(client, "getActiveValidators").mockResolvedValueOnce(validators);
      beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValueOnce(undefined);

      // Act
      const result = await client.getValidatorsForWithdrawalRequestsAscending();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith(
        "getValidatorsForWithdrawalRequestsAscending - failed to retrieve validators or pending withdrawals",
        { allValidators: false, pendingWithdrawalsQueue: true },
      );
    });

    it("returns undefined when joinValidatorsWithPendingWithdrawals returns undefined", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators = [createValidatorBalance()];
      jest.spyOn(client, "getActiveValidators").mockResolvedValueOnce(validators);
      jest.spyOn(client, "joinValidatorsWithPendingWithdrawals").mockReturnValueOnce(undefined);
      beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValueOnce([createPendingWithdrawal()]);

      // Act
      const result = await client.getValidatorsForWithdrawalRequestsAscending();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith(
        "getValidatorsForWithdrawalRequestsAscending - joinValidatorsWithPendingWithdrawals returned undefined",
      );
    });

    it("sorts validators ascending by withdrawable amount", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validatorHigh = createValidatorBalance({
        balance: VALIDATOR_45_ETH,
        validatorIndex: VALIDATOR_INDEX_10,
        publicKey: "validator-high",
      });
      const validatorLow = createValidatorBalance({
        balance: VALIDATOR_40_ETH,
        validatorIndex: 11n,
        publicKey: "validator-low",
      });
      jest.spyOn(client, "getActiveValidators").mockResolvedValueOnce([validatorHigh, validatorLow]);
      beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValueOnce([
        createPendingWithdrawal({ validator_index: 10, amount: WITHDRAWAL_2_ETH }),
        createPendingWithdrawal({ validator_index: 11, amount: WITHDRAWAL_6_ETH }),
      ]);
      beaconNodeApiClient.getCurrentEpoch.mockResolvedValueOnce(SHARD_COMMITTEE_PERIOD);

      // Act
      const result = await client.getValidatorsForWithdrawalRequestsAscending();

      // Assert
      const expectedHigh: ValidatorBalanceWithPendingWithdrawal = {
        ...validatorHigh,
        pendingWithdrawalAmount: WITHDRAWAL_2_ETH,
        withdrawableAmount: safeSub(safeSub(validatorHigh.balance, WITHDRAWAL_2_ETH), VALIDATOR_32_ETH),
      };
      const expectedLow: ValidatorBalanceWithPendingWithdrawal = {
        ...validatorLow,
        pendingWithdrawalAmount: WITHDRAWAL_6_ETH,
        withdrawableAmount: safeSub(safeSub(validatorLow.balance, WITHDRAWAL_6_ETH), VALIDATOR_32_ETH),
      };
      expect(result).toEqual([expectedLow, expectedHigh]);
    });

    it("skips eligibility filter and logs warning when getCurrentEpoch returns undefined", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validator = createValidatorBalance();
      jest.spyOn(client, "getActiveValidators").mockResolvedValueOnce([validator]);
      beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValueOnce([]);
      beaconNodeApiClient.getCurrentEpoch.mockResolvedValueOnce(undefined);

      // Act
      const result = await client.getValidatorsForWithdrawalRequestsAscending();

      // Assert
      expect(result).toBeDefined();
      expect(logger.warn).toHaveBeenCalledWith(
        "getValidatorsForWithdrawalRequestsAscending - failed to retrieve current epoch, skipping filter",
      );
    });

    it("filters out validators not active long enough based on shard committee period", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const eligibleValidator = createValidatorBalance({
        activationEpoch: EPOCH_0,
        publicKey: "validator-eligible",
      });
      const ineligibleValidator = createValidatorBalance({
        activationEpoch: EPOCH_100,
        validatorIndex: VALIDATOR_INDEX_2,
        publicKey: "validator-ineligible",
      });
      jest.spyOn(client, "getActiveValidators").mockResolvedValueOnce([eligibleValidator, ineligibleValidator]);
      beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValueOnce([]);
      beaconNodeApiClient.getCurrentEpoch.mockResolvedValueOnce(SHARD_COMMITTEE_PERIOD);

      // Act
      const result = await client.getValidatorsForWithdrawalRequestsAscending();

      // Assert
      const expectedEligible: ValidatorBalanceWithPendingWithdrawal = {
        ...eligibleValidator,
        pendingWithdrawalAmount: 0n,
        withdrawableAmount: safeSub(safeSub(eligibleValidator.balance, 0n), VALIDATOR_32_ETH),
      };
      expect(result).toEqual([expectedEligible]);
      expect(logger.info).toHaveBeenCalledWith("getValidatorsForWithdrawalRequestsAscending succeeded, validatorCount=1");
    });

    it("includes validators at exact shard committee period boundary", async () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validatorBoundary = createValidatorBalance({ activationEpoch: EPOCH_0 });
      jest.spyOn(client, "getActiveValidators").mockResolvedValueOnce([validatorBoundary]);
      beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValueOnce([]);
      beaconNodeApiClient.getCurrentEpoch.mockResolvedValueOnce(SHARD_COMMITTEE_PERIOD);

      // Act
      const result = await client.getValidatorsForWithdrawalRequestsAscending();

      // Assert
      expect(result).toBeDefined();
      expect(result).toHaveLength(1);
      expect(logger.info).toHaveBeenCalledWith("getValidatorsForWithdrawalRequestsAscending succeeded, validatorCount=1");
    });
  });

  describe("getFilteredAndAggregatedPendingWithdrawals", () => {
    it("returns undefined and logs warning when validators are undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const pendingWithdrawals = [createPendingWithdrawal()];

      // Act
      const result = client.getFilteredAndAggregatedPendingWithdrawals(undefined, pendingWithdrawals);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("getFilteredAndAggregatedPendingWithdrawals - invalid inputs", {
        allValidators: true,
        pendingWithdrawalsQueue: false,
      });
    });

    it("returns undefined and logs warning when pending withdrawals are undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators = [createValidatorBalance()];

      // Act
      const result = client.getFilteredAndAggregatedPendingWithdrawals(validators, undefined);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("getFilteredAndAggregatedPendingWithdrawals - invalid inputs", {
        allValidators: false,
        pendingWithdrawalsQueue: true,
      });
    });

    it("filters out withdrawals not matching active validators", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validator = createValidatorBalance({ validatorIndex: VALIDATOR_INDEX_1, publicKey: "validator-1" });
      const pendingWithdrawals = [
        createPendingWithdrawal({ validator_index: 1, amount: WITHDRAWAL_2_ETH, withdrawable_epoch: EPOCH_100 }),
        createPendingWithdrawal({ validator_index: 999, amount: WITHDRAWAL_5_ETH, withdrawable_epoch: EPOCH_200 }),
      ];

      // Act
      const result = client.getFilteredAndAggregatedPendingWithdrawals([validator], pendingWithdrawals);

      // Assert
      expect(result).toEqual([
        {
          validator_index: 1,
          withdrawable_epoch: EPOCH_100,
          amount: WITHDRAWAL_2_ETH,
          pubkey: "validator-1",
        },
      ]);
      expect(logger.debug).toHaveBeenCalledWith("getFilteredAndAggregatedPendingWithdrawals - filtered withdrawals", {
        totalPendingWithdrawals: 2,
        filteredCount: 1,
        uniqueValidatorIndices: 1,
      });
    });

    it("aggregates amounts by validator_index and withdrawable_epoch", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validator1 = createValidatorBalance({ validatorIndex: VALIDATOR_INDEX_1, publicKey: "validator-1" });
      const validator2 = createValidatorBalance({ validatorIndex: VALIDATOR_INDEX_2, publicKey: "validator-2" });
      const pendingWithdrawals = [
        createPendingWithdrawal({ validator_index: 1, amount: WITHDRAWAL_2_ETH, withdrawable_epoch: EPOCH_100 }),
        createPendingWithdrawal({ validator_index: 1, amount: WITHDRAWAL_3_ETH, withdrawable_epoch: EPOCH_100 }),
        createPendingWithdrawal({ validator_index: 1, amount: WITHDRAWAL_5_ETH, withdrawable_epoch: EPOCH_200 }),
        createPendingWithdrawal({ validator_index: 2, amount: WITHDRAWAL_1_ETH, withdrawable_epoch: EPOCH_100 }),
      ];

      // Act
      const result = client.getFilteredAndAggregatedPendingWithdrawals([validator1, validator2], pendingWithdrawals);

      // Assert
      expect(result).toEqual(
        expect.arrayContaining([
          {
            validator_index: 1,
            withdrawable_epoch: EPOCH_100,
            amount: WITHDRAWAL_5_ETH,
            pubkey: "validator-1",
          },
          {
            validator_index: 1,
            withdrawable_epoch: EPOCH_200,
            amount: WITHDRAWAL_5_ETH,
            pubkey: "validator-1",
          },
          {
            validator_index: 2,
            withdrawable_epoch: EPOCH_100,
            amount: WITHDRAWAL_1_ETH,
            pubkey: "validator-2",
          },
        ]),
      );
      expect(result).toHaveLength(3);
      expect(logger.info).toHaveBeenCalledWith("getFilteredAndAggregatedPendingWithdrawals succeeded, aggregatedCount=3");
    });

    it("returns empty array when validators array is empty", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const pendingWithdrawals = [createPendingWithdrawal()];

      // Act
      const result = client.getFilteredAndAggregatedPendingWithdrawals([], pendingWithdrawals);

      // Assert
      expect(result).toEqual([]);
      expect(logger.info).toHaveBeenCalledWith("getFilteredAndAggregatedPendingWithdrawals succeeded, aggregatedCount=0");
    });
  });

  describe("getTotalPendingPartialWithdrawalsWei", () => {
    it("calculates total pending withdrawals in wei", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators: ValidatorBalanceWithPendingWithdrawal[] = [
        {
          ...createValidatorBalance(),
          pendingWithdrawalAmount: WITHDRAWAL_3_ETH,
          withdrawableAmount: 0n,
        },
        {
          ...createValidatorBalance({ validatorIndex: VALIDATOR_INDEX_2 }),
          pendingWithdrawalAmount: WITHDRAWAL_1_ETH,
          withdrawableAmount: 0n,
        },
      ];

      // Act
      const totalWei = client.getTotalPendingPartialWithdrawalsWei(validators);

      // Assert
      expect(totalWei).toBe(WITHDRAWAL_4_ETH * ONE_GWEI);
      expect(logger.info).toHaveBeenCalledWith("getTotalPendingPartialWithdrawalsWei totalWei=4000000000000000000");
    });
  });

  describe("getTotalValidatorBalanceGwei", () => {
    it("returns undefined when input is undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getTotalValidatorBalanceGwei(undefined);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.info).not.toHaveBeenCalled();
    });

    it("returns undefined when input is empty array", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getTotalValidatorBalanceGwei([]);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.info).not.toHaveBeenCalled();
    });

    it("sums balances from multiple validators", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators = [
        createValidatorBalance({ balance: VALIDATOR_32_ETH }),
        createValidatorBalance({ balance: VALIDATOR_40_ETH, validatorIndex: VALIDATOR_INDEX_2 }),
        createValidatorBalance({ balance: VALIDATOR_35_ETH, validatorIndex: 3n }),
      ];

      // Act
      const totalGwei = client.getTotalValidatorBalanceGwei(validators);

      // Assert
      expect(totalGwei).toBe(107n * ONE_GWEI);
      expect(logger.info).toHaveBeenCalledWith("getTotalValidatorBalanceGwei totalGwei=107000000000");
    });
  });

  describe("getSlashedValidators", () => {
    it("returns undefined and logs warning when input is undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getSlashedValidators(undefined);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("getSlashedValidators - invalid input: validators is undefined");
    });

    it("returns empty array when input is empty array", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getSlashedValidators([]);

      // Assert
      expect(result).toEqual([]);
      expect(logger.info).toHaveBeenCalledWith("getSlashedValidators succeeded, slashedCount=0");
    });

    it("filters and returns only slashed validators", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const slashedValidator = createExitingValidator({ publicKey: "validator-slashed", slashed: true });
      const normalValidator = createExitingValidator({
        publicKey: "validator-normal",
        validatorIndex: VALIDATOR_INDEX_2,
        slashed: false,
      });

      // Act
      const result = client.getSlashedValidators([slashedValidator, normalValidator]);

      // Assert
      expect(result).toEqual([slashedValidator]);
      expect(logger.info).toHaveBeenCalledWith("getSlashedValidators succeeded, slashedCount=1");
    });
  });

  describe("getNonSlashedAndExitingValidators", () => {
    it("returns undefined and logs warning when input is undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getNonSlashedAndExitingValidators(undefined);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("getNonSlashedAndExitingValidators - invalid input: validators is undefined");
    });

    it("returns empty array when input is empty array", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getNonSlashedAndExitingValidators([]);

      // Assert
      expect(result).toEqual([]);
      expect(logger.info).toHaveBeenCalledWith("getNonSlashedAndExitingValidators succeeded, nonSlashedCount=0");
    });

    it("filters and returns only non-slashed validators", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const slashedValidator = createExitingValidator({ publicKey: "validator-slashed", slashed: true });
      const normalValidator = createExitingValidator({
        publicKey: "validator-normal",
        validatorIndex: VALIDATOR_INDEX_2,
        slashed: false,
      });

      // Act
      const result = client.getNonSlashedAndExitingValidators([slashedValidator, normalValidator]);

      // Assert
      expect(result).toEqual([normalValidator]);
      expect(logger.info).toHaveBeenCalledWith("getNonSlashedAndExitingValidators succeeded, nonSlashedCount=1");
    });
  });

  describe("getTotalBalanceOfExitingValidators", () => {
    it("returns undefined and logs warning when input is undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getTotalBalanceOfExitingValidators(undefined);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("getTotalBalanceOfExitingValidators - invalid input: validators is undefined", {
        validators: true,
      });
    });

    it("returns 0n when input is empty array", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getTotalBalanceOfExitingValidators([]);

      // Assert
      expect(result).toBe(0n);
      expect(logger.info).toHaveBeenCalledWith("getTotalBalanceOfExitingValidators - empty array, returning 0");
    });

    it("sums balances from multiple exiting validators", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators = [
        createExitingValidator({ balance: VALIDATOR_32_ETH }),
        createExitingValidator({ balance: VALIDATOR_40_ETH, validatorIndex: VALIDATOR_INDEX_2 }),
        createExitingValidator({ balance: VALIDATOR_35_ETH, validatorIndex: 3n }),
      ];

      // Act
      const totalGwei = client.getTotalBalanceOfExitingValidators(validators);

      // Assert
      expect(totalGwei).toBe(107n * ONE_GWEI);
      expect(logger.info).toHaveBeenCalledWith("getTotalBalanceOfExitingValidators totalGwei=107000000000");
    });
  });

  describe("getTotalBalanceOfExitedValidators", () => {
    it("returns undefined and logs warning when input is undefined", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getTotalBalanceOfExitedValidators(undefined);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.warn).toHaveBeenCalledWith("getTotalBalanceOfExitedValidators - invalid input: validators is undefined", {
        validators: true,
      });
    });

    it("returns 0n when input is empty array", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);

      // Act
      const result = client.getTotalBalanceOfExitedValidators([]);

      // Assert
      expect(result).toBe(0n);
      expect(logger.info).toHaveBeenCalledWith("getTotalBalanceOfExitedValidators - empty array, returning 0");
    });

    it("sums balances from multiple exited validators", () => {
      // Arrange
      const logger = createLoggerMock();
      const retryService = createRetryService();
      const apolloClient = createApolloClient();
      const beaconNodeApiClient = createBeaconNodeApiClient();
      const client = new ConsensysStakingApiClient(logger, retryService, apolloClient, beaconNodeApiClient);
      const validators = [
        createExitedValidator({ balance: VALIDATOR_32_ETH }),
        createExitedValidator({ balance: VALIDATOR_40_ETH, validatorIndex: VALIDATOR_INDEX_2 }),
        createExitedValidator({ balance: VALIDATOR_35_ETH, validatorIndex: 3n }),
      ];

      // Act
      const totalGwei = client.getTotalBalanceOfExitedValidators(validators);

      // Assert
      expect(totalGwei).toBe(107n * ONE_GWEI);
      expect(logger.info).toHaveBeenCalledWith("getTotalBalanceOfExitedValidators totalGwei=107000000000");
    });
  });
});
