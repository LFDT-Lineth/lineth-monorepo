import { jest } from "@jest/globals";
import { MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE, ONE_GWEI } from "@consensys/linea-shared-utils";
import type { Address, Hex, TransactionReceipt } from "viem";

import { BeaconChainStakingClient } from "../BeaconChainStakingClient.js";
import { createLoggerMock, createMetricsUpdaterMock } from "../../__tests__/helpers/index.js";
import type { IValidatorDataClient } from "../../core/clients/IValidatorDataClient.js";
import type { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import type {
  ExitedValidator,
  ExitingValidator,
  ValidatorBalance,
  ValidatorBalanceWithPendingWithdrawal,
} from "../../core/entities/Validator.js";
import type { WithdrawalRequests } from "../../core/entities/LidoStakingVaultWithdrawalParams.js";
import type { PendingPartialWithdrawal } from "@consensys/linea-shared-utils";

// Test constants
const YIELD_PROVIDER = "0xyieldprovider" as Address;
const DEFAULT_MAX_VALIDATORS_PER_TX = 3;
const DEFAULT_MIN_WITHDRAWAL_THRESHOLD_ETH = 0n;

// Factory functions
const createValidatorDataClientMock = () => {
  const getActiveValidators = jest.fn<() => Promise<ValidatorBalance[] | undefined>>();
  const getExitingValidators = jest.fn<() => Promise<ExitingValidator[] | undefined>>();
  const getValidatorsForWithdrawalRequestsAscending =
    jest.fn<() => Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined>>();
  const joinValidatorsWithPendingWithdrawals = jest.fn<
    (
      allValidators: ValidatorBalance[] | undefined,
      pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
    ) => ValidatorBalanceWithPendingWithdrawal[] | undefined
  >();
  const getFilteredAndAggregatedPendingWithdrawals = jest.fn<
    (
      allValidators: ValidatorBalance[] | undefined,
      pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
    ) => undefined
  >();
  const getTotalPendingPartialWithdrawalsWei = jest
    .fn<(validatorList: ValidatorBalanceWithPendingWithdrawal[]) => bigint>()
    .mockReturnValue(0n);
  const getTotalValidatorBalanceGwei = jest
    .fn<(validators: ValidatorBalance[] | undefined) => bigint | undefined>()
    .mockReturnValue(undefined);
  const getTotalBalanceOfExitingValidators = jest
    .fn<(validators: ExitingValidator[] | undefined) => bigint | undefined>()
    .mockReturnValue(undefined);
  const getExitedValidators = jest.fn<() => Promise<ExitedValidator[] | undefined>>();
  const getTotalBalanceOfExitedValidators = jest
    .fn<(validators: ExitedValidator[] | undefined) => bigint | undefined>()
    .mockReturnValue(undefined);

  const client: IValidatorDataClient = {
    getActiveValidators,
    getExitingValidators,
    getExitedValidators,
    getValidatorsForWithdrawalRequestsAscending,
    joinValidatorsWithPendingWithdrawals,
    getFilteredAndAggregatedPendingWithdrawals,
    getTotalPendingPartialWithdrawalsWei,
    getTotalValidatorBalanceGwei,
    getTotalBalanceOfExitingValidators,
    getTotalBalanceOfExitedValidators,
  };

  return {
    client,
    getActiveValidators,
    getExitingValidators,
    getExitedValidators,
    getValidatorsForWithdrawalRequestsAscending,
    joinValidatorsWithPendingWithdrawals,
    getFilteredAndAggregatedPendingWithdrawals,
    getTotalPendingPartialWithdrawalsWei,
    getTotalValidatorBalanceGwei,
    getTotalBalanceOfExitingValidators,
    getTotalBalanceOfExitedValidators,
  };
};

const createYieldManagerMock = () => {
  const unstakeMock = jest.fn(async (_: Address, __: WithdrawalRequests) => ({}) as TransactionReceipt);
  const mock = {
    unstake: unstakeMock,
  } as unknown as IYieldManager<TransactionReceipt>;
  return { mock, unstakeMock };
};

const createValidator = (
  overrides: Partial<ValidatorBalanceWithPendingWithdrawal> & Pick<ValidatorBalanceWithPendingWithdrawal, "publicKey">,
): ValidatorBalanceWithPendingWithdrawal => ({
  balance: 32n,
  effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE,
  pendingWithdrawalAmount: 0n,
  withdrawableAmount: 0n,
  validatorIndex: 0n,
  activationEpoch: 0,
  ...overrides,
});

describe("BeaconChainStakingClient", () => {
  let client: BeaconChainStakingClient;
  let logger: ReturnType<typeof createLoggerMock>;
  let metricsUpdater: ReturnType<typeof createMetricsUpdaterMock>;
  let validatorDataClient: IValidatorDataClient;
  let yieldManagerContractClient: IYieldManager<TransactionReceipt>;
  let getValidatorsForWithdrawalRequestsAscending: jest.MockedFunction<
    () => Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined>
  >;
  let getTotalPendingPartialWithdrawalsWei: jest.MockedFunction<
    (validatorList: ValidatorBalanceWithPendingWithdrawal[]) => bigint
  >;
  let unstakeMock: jest.MockedFunction<(provider: Address, requests: WithdrawalRequests) => Promise<TransactionReceipt>>;

  const setupClient = (
    maxValidatorsPerTx = DEFAULT_MAX_VALIDATORS_PER_TX,
    minWithdrawalThresholdEth = DEFAULT_MIN_WITHDRAWAL_THRESHOLD_ETH,
  ) => {
    logger = createLoggerMock();
    metricsUpdater = createMetricsUpdaterMock();
    const validatorDataClientMock = createValidatorDataClientMock();
    validatorDataClient = validatorDataClientMock.client;
    getValidatorsForWithdrawalRequestsAscending = validatorDataClientMock.getValidatorsForWithdrawalRequestsAscending;
    getTotalPendingPartialWithdrawalsWei = validatorDataClientMock.getTotalPendingPartialWithdrawalsWei;

    const yieldManagerMock = createYieldManagerMock();
    yieldManagerContractClient = yieldManagerMock.mock;
    unstakeMock = yieldManagerMock.unstakeMock;

    client = new BeaconChainStakingClient(
      logger,
      metricsUpdater,
      validatorDataClient,
      maxValidatorsPerTx,
      yieldManagerContractClient,
      YIELD_PROVIDER,
      minWithdrawalThresholdEth,
    );
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("submitWithdrawalRequestsToFulfilAmount", () => {
    it("logs error when validator data is unavailable", async () => {
      // Arrange
      setupClient();
      getValidatorsForWithdrawalRequestsAscending.mockResolvedValueOnce(undefined);

      // Act
      await client.submitWithdrawalRequestsToFulfilAmount(10n);

      // Assert
      expect(logger.error).toHaveBeenCalledWith(
        "submitWithdrawalRequestsToFulfilAmount failed to get sortedValidatorList with pending withdrawals",
      );
      expect(getTotalPendingPartialWithdrawalsWei).not.toHaveBeenCalled();
      expect(unstakeMock).not.toHaveBeenCalled();
    });

    it("skips submission when pending withdrawals already cover amount", async () => {
      // Arrange
      setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 3n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 1n }),
      ];
      const amountWei = 4n * ONE_GWEI;
      getValidatorsForWithdrawalRequestsAscending.mockResolvedValueOnce(validators);
      getTotalPendingPartialWithdrawalsWei.mockReturnValueOnce(amountWei);

      // Act
      await client.submitWithdrawalRequestsToFulfilAmount(amountWei);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("submitWithdrawalRequestsToFulfilAmount - no remaining withdrawal amount needed"),
      );
      expect(getTotalPendingPartialWithdrawalsWei).toHaveBeenCalledWith(validators);
      expect(metricsUpdater.setLastTotalPendingPartialWithdrawalsGwei).toHaveBeenCalledWith(4);
      expect(unstakeMock).not.toHaveBeenCalled();
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).not.toHaveBeenCalled();
    });

    it("submits partial withdrawal requests up to configured limit and records metrics", async () => {
      // Arrange
      const maxValidatorsPerTx = 2;
      setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 2n }),
        createValidator({ publicKey: "validator-3", withdrawableAmount: 3n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 5n }),
      ];
      const amountWei = 3n * ONE_GWEI;
      getValidatorsForWithdrawalRequestsAscending.mockResolvedValueOnce(validators);
      getTotalPendingPartialWithdrawalsWei.mockReturnValueOnce(0n);

      // Act
      await client.submitWithdrawalRequestsToFulfilAmount(amountWei);

      // Assert
      expect(getTotalPendingPartialWithdrawalsWei).toHaveBeenCalledWith(validators);
      expect(metricsUpdater.setLastTotalPendingPartialWithdrawalsGwei).toHaveBeenCalledWith(0);
      expect(unstakeMock).toHaveBeenCalledTimes(1);

      const [, withdrawalRequests] = unstakeMock.mock.calls[0];
      expect(withdrawalRequests.pubkeys).toEqual(["validator-1" as Hex, "validator-3" as Hex]);
      expect(withdrawalRequests.amountsGwei).toEqual([2n, 1n]);

      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(1, "validator-1" as Hex, 2);
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(2, "validator-3" as Hex, 1);
    });
  });

  describe("_submitPartialWithdrawalRequests (private)", () => {
    it("returns max validator slots and skips unstake when validator list is empty", async () => {
      // Arrange
      const maxValidatorsPerTx = 3;
      setupClient(maxValidatorsPerTx);

      // Act
      const remaining = await (
        client as unknown as {
          _submitPartialWithdrawalRequests(
            list: ValidatorBalanceWithPendingWithdrawal[],
            amountWei: bigint,
          ): Promise<number>;
        }
      )._submitPartialWithdrawalRequests([], 1n * ONE_GWEI);

      // Assert
      expect(remaining).toBe(maxValidatorsPerTx);
      expect(logger.info).toHaveBeenCalledWith(
        "_submitPartialWithdrawalRequests - sortedValidatorList is empty, returning max withdrawal requests",
      );
      expect(unstakeMock).not.toHaveBeenCalled();
    });

    it("returns max validator slots when no validator has withdrawable balance", async () => {
      // Arrange
      const maxValidatorsPerTx = 3;
      setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 0n }),
      ];

      // Act
      const remaining = await (
        client as unknown as {
          _submitPartialWithdrawalRequests(
            list: ValidatorBalanceWithPendingWithdrawal[],
            amountWei: bigint,
          ): Promise<number>;
        }
      )._submitPartialWithdrawalRequests(validators, 5n * ONE_GWEI);

      // Assert
      expect(remaining).toBe(maxValidatorsPerTx);
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("_submitPartialWithdrawalRequests - no withdrawal requests to submit"),
      );
      expect(unstakeMock).not.toHaveBeenCalled();
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).not.toHaveBeenCalled();
    });

    it("stops building requests once required amount is met", async () => {
      // Arrange
      const maxValidatorsPerTx = 3;
      setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 5n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 5n }),
      ];

      // Act
      const remaining = await (
        client as unknown as {
          _submitPartialWithdrawalRequests(
            list: ValidatorBalanceWithPendingWithdrawal[],
            amountWei: bigint,
          ): Promise<number>;
        }
      )._submitPartialWithdrawalRequests(validators, 1n * ONE_GWEI);

      // Assert
      expect(remaining).toBe(maxValidatorsPerTx - 1);
      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual(["validator-1" as Hex]);
      expect(requests.amountsGwei).toEqual([1n]);
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenCalledTimes(1);
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenCalledWith("validator-1" as Hex, 1);
    });

    it("filters out withdrawal requests below minimum threshold", async () => {
      // Arrange
      const maxValidatorsPerTx = 3;
      const minWithdrawalThresholdEth = 1n;
      setupClient(maxValidatorsPerTx, minWithdrawalThresholdEth);
      const minWithdrawalThresholdGwei = minWithdrawalThresholdEth * ONE_GWEI;
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: minWithdrawalThresholdGwei - 1n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: minWithdrawalThresholdGwei }),
        createValidator({ publicKey: "validator-3", withdrawableAmount: minWithdrawalThresholdGwei + 1n }),
      ];

      // Act
      const remaining = await (
        client as unknown as {
          _submitPartialWithdrawalRequests(
            list: ValidatorBalanceWithPendingWithdrawal[],
            amountWei: bigint,
          ): Promise<number>;
        }
      )._submitPartialWithdrawalRequests(validators, 10n * ONE_GWEI * ONE_GWEI);

      // Assert
      expect(remaining).toBe(maxValidatorsPerTx - 1);
      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual(["validator-3" as Hex]);
      expect(requests.amountsGwei).toEqual([minWithdrawalThresholdGwei + 1n]);
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenCalledTimes(1);
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenCalledWith(
        "validator-3" as Hex,
        Number(minWithdrawalThresholdGwei + 1n),
      );
    });
  });

  describe("submitMaxAvailableWithdrawalRequests", () => {
    it("logs error when validator data is unavailable", async () => {
      // Arrange
      setupClient();
      getValidatorsForWithdrawalRequestsAscending.mockResolvedValueOnce(undefined);

      // Act
      await client.submitMaxAvailableWithdrawalRequests();

      // Assert
      expect(logger.error).toHaveBeenCalledWith(
        "submitMaxAvailableWithdrawalRequests failed to get sortedValidatorList with pending withdrawals",
      );
      expect(unstakeMock).not.toHaveBeenCalled();
    });

    it("submits partial withdrawals and validator exits using remaining slots", async () => {
      // Arrange
      const maxValidatorsPerTx = 3;
      setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 2n, effectiveBalance: 33n * ONE_GWEI }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 3n, effectiveBalance: 34n * ONE_GWEI }),
        createValidator({ publicKey: "validator-3", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
        createValidator({ publicKey: "validator-4", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
      ];
      getValidatorsForWithdrawalRequestsAscending.mockResolvedValueOnce(validators);

      // Act
      await client.submitMaxAvailableWithdrawalRequests();

      // Assert
      expect(unstakeMock).toHaveBeenCalledTimes(2);

      const [, partialRequests] = unstakeMock.mock.calls[0];
      expect(partialRequests.pubkeys).toEqual(["validator-1" as Hex, "validator-2" as Hex]);
      expect(partialRequests.amountsGwei).toEqual([2n, 3n]);

      const [, exitRequests] = unstakeMock.mock.calls[1];
      expect(exitRequests.pubkeys).toEqual(["validator-3" as Hex]);
      expect(exitRequests.amountsGwei).toEqual([]);

      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenCalledTimes(2);
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(1, "validator-1" as Hex, 2);
      expect(metricsUpdater.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(2, "validator-2" as Hex, 3);

      expect(metricsUpdater.incrementValidatorExit).toHaveBeenCalledTimes(1);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenCalledWith("validator-3" as Hex);
    });
  });

  describe("_submitValidatorExits (private)", () => {
    it("returns immediately when no withdrawal slots remain", async () => {
      // Arrange
      setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 0n }),
      ];

      // Act
      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, 0);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        "_submitValidatorExits - no remaining withdrawals or empty validator list, skipping",
        { remainingWithdrawals: 0, validatorListLength: 2 },
      );
      expect(unstakeMock).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementValidatorExit).not.toHaveBeenCalled();
    });

    it("returns without unstaking when no validators qualify for exits", async () => {
      // Arrange
      setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", effectiveBalance: 33n * ONE_GWEI }),
        createValidator({ publicKey: "validator-2", effectiveBalance: 34n * ONE_GWEI }),
      ];

      // Act
      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, 2);

      // Assert
      expect(logger.info).toHaveBeenCalledWith("_submitValidatorExits - no validators to exit, skipping unstake");
      expect(unstakeMock).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementValidatorExit).not.toHaveBeenCalled();
    });

    it("stops adding exits when reaching remainingWithdrawals limit", async () => {
      // Arrange
      const maxValidatorsPerTx = 5;
      const remainingWithdrawals = 2;
      setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
        createValidator({ publicKey: "validator-2", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
        createValidator({ publicKey: "validator-3", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
      ];

      // Act
      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, remainingWithdrawals);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("_submitValidatorExits - reached remainingWithdrawals limit, breaking loop"),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`withdrawalRequests.pubkeys.length=${remainingWithdrawals}`),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`remainingWithdrawals=${remainingWithdrawals}`),
      );
      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual(["validator-1" as Hex, "validator-2" as Hex]);
      expect(requests.amountsGwei).toEqual([]);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenCalledTimes(2);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenNthCalledWith(1, "validator-1" as Hex);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenNthCalledWith(2, "validator-2" as Hex);
    });

    it("stops adding exits when reaching maximum per-transaction limit", async () => {
      // Arrange
      const maxValidatorsPerTx = 3;
      setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
        createValidator({ publicKey: "validator-2", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
        createValidator({ publicKey: "validator-3", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
        createValidator({ publicKey: "validator-4", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
      ];

      // Act
      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, maxValidatorsPerTx + 1);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("_submitValidatorExits - reached maxValidatorWithdrawalRequestsPerTransaction limit, breaking loop"),
      );
      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual([
        "validator-1" as Hex,
        "validator-2" as Hex,
        "validator-3" as Hex,
      ]);
      expect(requests.amountsGwei).toEqual([]);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenCalledTimes(3);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenNthCalledWith(1, "validator-1" as Hex);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenNthCalledWith(2, "validator-2" as Hex);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenNthCalledWith(3, "validator-3" as Hex);
    });

    it("skips validators with pending withdrawal amounts", async () => {
      // Arrange
      setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE, pendingWithdrawalAmount: 5n }),
        createValidator({ publicKey: "validator-2", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE, pendingWithdrawalAmount: 10n }),
      ];

      // Act
      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, 2);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("_submitValidatorExits - skipping validator with pending withdrawal, continuing loop"),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("pubkey=validator-1"),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("pubkey=validator-2"),
      );
      expect(logger.info).toHaveBeenCalledWith("_submitValidatorExits - no validators to exit, skipping unstake");
      expect(unstakeMock).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementValidatorExit).not.toHaveBeenCalled();
    });

    it("processes validators without pending withdrawals and skips those with pending withdrawals", async () => {
      // Arrange
      setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE, pendingWithdrawalAmount: 5n }),
        createValidator({ publicKey: "validator-2", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE, pendingWithdrawalAmount: 0n }),
        createValidator({ publicKey: "validator-3", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE, pendingWithdrawalAmount: 10n }),
        createValidator({ publicKey: "validator-4", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE, pendingWithdrawalAmount: 0n }),
      ];

      // Act
      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, 5);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("_submitValidatorExits - skipping validator with pending withdrawal, continuing loop"),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("pubkey=validator-1"),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining("pubkey=validator-3"),
      );
      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual(["validator-2" as Hex, "validator-4" as Hex]);
      expect(requests.amountsGwei).toEqual([]);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenCalledTimes(2);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenNthCalledWith(1, "validator-2" as Hex);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenNthCalledWith(2, "validator-4" as Hex);
    });

    it("skips validators with effectiveBalance not equal to minimum", async () => {
      // Arrange
      setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", effectiveBalance: 33n * ONE_GWEI }),
        createValidator({ publicKey: "validator-2", effectiveBalance: 34n * ONE_GWEI }),
        createValidator({ publicKey: "validator-3", effectiveBalance: MINIMUM_0X02_VALIDATOR_EFFECTIVE_BALANCE }),
      ];

      // Act
      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, 5);

      // Assert
      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual(["validator-3" as Hex]);
      expect(requests.amountsGwei).toEqual([]);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenCalledTimes(1);
      expect(metricsUpdater.incrementValidatorExit).toHaveBeenCalledWith("validator-3" as Hex);
    });
  });
});
