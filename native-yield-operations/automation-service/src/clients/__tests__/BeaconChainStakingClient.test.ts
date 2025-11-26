import { jest } from "@jest/globals";
import { BeaconChainStakingClient } from "../BeaconChainStakingClient.js";
import type { ILogger } from "@consensys/linea-shared-utils";
import type { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IValidatorDataClient } from "../../core/clients/IValidatorDataClient.js";
import type { ValidatorBalance, ValidatorBalanceWithPendingWithdrawal } from "../../core/entities/ValidatorBalance.js";
import type { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import type { Address, TransactionReceipt } from "viem";
import { stringToHex } from "viem";
import { ONE_GWEI } from "@consensys/linea-shared-utils";
import type { WithdrawalRequests } from "../../core/entities/LidoStakingVaultWithdrawalParams.js";

const YIELD_PROVIDER = "0xyieldprovider" as Address;

const createLoggerMock = (): ILogger => ({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

const createMetricsUpdaterMock = () => {
  const addValidatorPartialUnstakeAmount = jest.fn();
  const incrementValidatorExit = jest.fn();

  const metricsUpdater: INativeYieldAutomationMetricsUpdater = {
    recordRebalance: jest.fn(),
    addValidatorPartialUnstakeAmount,
    incrementValidatorExit,
    incrementLidoVaultAccountingReport: jest.fn(),
    incrementReportYield: jest.fn(),
    addReportedYieldAmount: jest.fn(),
    setLastPeekedNegativeYieldReport: jest.fn(async () => undefined),
    setLastPeekedPositiveYieldReport: jest.fn(async () => undefined),
    setLastPeekUnpaidLidoProtocolFees: jest.fn(async () => undefined),
    addNodeOperatorFeesPaid: jest.fn(),
    addLiabilitiesPaid: jest.fn(),
    addLidoFeesPaid: jest.fn(),
    incrementOperationModeTrigger: jest.fn(),
    incrementOperationModeExecution: jest.fn(),
    recordOperationModeDuration: jest.fn(),
  };

  return { metricsUpdater, addValidatorPartialUnstakeAmount, incrementValidatorExit };
};

const createValidatorDataClientMock = () => {
  const getActiveValidators = jest.fn<() => Promise<ValidatorBalance[] | undefined>>();
  const getActiveValidatorsWithPendingWithdrawals =
    jest.fn<() => Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined>>();
  const getTotalPendingPartialWithdrawalsWei = jest
    .fn<(validatorList: ValidatorBalanceWithPendingWithdrawal[]) => bigint>()
    .mockReturnValue(0n);

  const client: IValidatorDataClient = {
    getActiveValidators,
    getActiveValidatorsWithPendingWithdrawals,
    getTotalPendingPartialWithdrawalsWei,
  };

  return {
    client,
    getActiveValidators,
    getActiveValidatorsWithPendingWithdrawals,
    getTotalPendingPartialWithdrawalsWei,
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
  effectiveBalance: 32n,
  pendingWithdrawalAmount: 0n,
  withdrawableAmount: 0n,
  validatorIndex: 0n,
  ...overrides,
});

describe("BeaconChainStakingClient", () => {
  const setupClient = (maxValidatorsPerTx = 3) => {
    const logger = createLoggerMock();
    const { metricsUpdater, addValidatorPartialUnstakeAmount, incrementValidatorExit } = createMetricsUpdaterMock();
    const {
      client: validatorDataClient,
      getActiveValidatorsWithPendingWithdrawals,
      getTotalPendingPartialWithdrawalsWei,
    } = createValidatorDataClientMock();
    const { mock: yieldManagerContractClient, unstakeMock } = createYieldManagerMock();

    const client = new BeaconChainStakingClient(
      logger,
      metricsUpdater,
      validatorDataClient,
      maxValidatorsPerTx,
      yieldManagerContractClient,
      YIELD_PROVIDER,
    );

    return {
      client,
      logger,
      metricsUpdater,
      validatorDataClient,
      unstakeMock,
      mocks: {
        addValidatorPartialUnstakeAmount,
        incrementValidatorExit,
        getActiveValidatorsWithPendingWithdrawals,
        getTotalPendingPartialWithdrawalsWei,
      },
    };
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("submitWithdrawalRequestsToFulfilAmount", () => {
    it("logs an error when validator data is unavailable", async () => {
      const { client, logger, unstakeMock, mocks } = setupClient();
      mocks.getActiveValidatorsWithPendingWithdrawals.mockResolvedValueOnce(undefined);

      await client.submitWithdrawalRequestsToFulfilAmount(10n);

      expect(logger.error).toHaveBeenCalledWith(
        "submitWithdrawalRequestsToFulfilAmount failed to get sortedValidatorList with pending withdrawals",
      );
      expect(mocks.getTotalPendingPartialWithdrawalsWei).not.toHaveBeenCalled();
      expect(unstakeMock).not.toHaveBeenCalled();
    });

    it("skips submission when pending withdrawals already cover the amount", async () => {
      const { client, unstakeMock, mocks } = setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 3n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 1n }),
      ];
      mocks.getActiveValidatorsWithPendingWithdrawals.mockResolvedValueOnce(validators);
      const amountWei = 4n * ONE_GWEI;
      mocks.getTotalPendingPartialWithdrawalsWei.mockReturnValueOnce(amountWei);

      await client.submitWithdrawalRequestsToFulfilAmount(amountWei);

      expect(mocks.getTotalPendingPartialWithdrawalsWei).toHaveBeenCalledWith(validators);
      expect(unstakeMock).not.toHaveBeenCalled();
      expect(mocks.addValidatorPartialUnstakeAmount).not.toHaveBeenCalled();
    });

    it("submits partial withdrawal requests up to the configured limit and records metrics", async () => {
      const maxValidatorsPerTx = 2;
      const { client, unstakeMock, mocks } = setupClient(maxValidatorsPerTx);

      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 2n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 5n }),
        createValidator({ publicKey: "validator-3", withdrawableAmount: 3n }),
      ];
      mocks.getActiveValidatorsWithPendingWithdrawals.mockResolvedValueOnce(validators);
      mocks.getTotalPendingPartialWithdrawalsWei.mockReturnValueOnce(0n);
      const amountWei = 3n * ONE_GWEI;

      await client.submitWithdrawalRequestsToFulfilAmount(amountWei);

      expect(mocks.getTotalPendingPartialWithdrawalsWei).toHaveBeenCalledWith(validators);
      expect(unstakeMock).toHaveBeenCalledTimes(1);

      const [, withdrawalRequests] = unstakeMock.mock.calls[0];
      expect(withdrawalRequests.pubkeys).toEqual([stringToHex("validator-1"), stringToHex("validator-2")]);
      expect(withdrawalRequests.amountsGwei).toEqual([2n, 1n]);

      expect(mocks.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(1, stringToHex("validator-1"), 2);
      expect(mocks.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(2, stringToHex("validator-2"), 1);
    });
  });

  describe("_submitPartialWithdrawalRequests (private)", () => {
    it("returns max validator slots and skips unstake when the validator list is empty", async () => {
      const maxValidatorsPerTx = 3;
      const { client, unstakeMock } = setupClient(maxValidatorsPerTx);

      const remaining = await (
        client as unknown as {
          _submitPartialWithdrawalRequests(
            list: ValidatorBalanceWithPendingWithdrawal[],
            amountWei: bigint,
          ): Promise<number>;
        }
      )._submitPartialWithdrawalRequests([], 1n * ONE_GWEI);

      expect(remaining).toBe(maxValidatorsPerTx);
      expect(unstakeMock).not.toHaveBeenCalled();
    });

    it("returns max validator slots when no validator has withdrawable balance", async () => {
      const maxValidatorsPerTx = 3;
      const { client, unstakeMock, mocks } = setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 0n }),
      ];

      const remaining = await (
        client as unknown as {
          _submitPartialWithdrawalRequests(
            list: ValidatorBalanceWithPendingWithdrawal[],
            amountWei: bigint,
          ): Promise<number>;
        }
      )._submitPartialWithdrawalRequests(validators, 5n * ONE_GWEI);

      expect(remaining).toBe(maxValidatorsPerTx);
      expect(unstakeMock).not.toHaveBeenCalled();
      expect(mocks.addValidatorPartialUnstakeAmount).not.toHaveBeenCalled();
    });

    it("stops building requests once the required amount is met even if the validator limit is not reached", async () => {
      const maxValidatorsPerTx = 3;
      const { client, unstakeMock, mocks } = setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 5n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 5n }),
      ];

      const remaining = await (
        client as unknown as {
          _submitPartialWithdrawalRequests(
            list: ValidatorBalanceWithPendingWithdrawal[],
            amountWei: bigint,
          ): Promise<number>;
        }
      )._submitPartialWithdrawalRequests(validators, 1n * ONE_GWEI);

      expect(remaining).toBe(maxValidatorsPerTx - 1);
      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual([stringToHex("validator-1")]);
      expect(requests.amountsGwei).toEqual([1n]);
      expect(mocks.addValidatorPartialUnstakeAmount).toHaveBeenCalledTimes(1);
      expect(mocks.addValidatorPartialUnstakeAmount).toHaveBeenCalledWith(stringToHex("validator-1"), 1);
    });
  });

  describe("submitMaxAvailableWithdrawalRequests", () => {
    it("logs an error when validator data is unavailable", async () => {
      const { client, logger, unstakeMock, mocks } = setupClient();
      mocks.getActiveValidatorsWithPendingWithdrawals.mockResolvedValueOnce(undefined);

      await client.submitMaxAvailableWithdrawalRequests();

      expect(logger.error).toHaveBeenCalledWith(
        "submitMaxAvailableWithdrawalRequests failed to get sortedValidatorList with pending withdrawals",
      );
      expect(unstakeMock).not.toHaveBeenCalled();
    });

    it("submits partial withdrawals and validator exits using remaining slots", async () => {
      const maxValidatorsPerTx = 3;
      const { client, unstakeMock, mocks } = setupClient(maxValidatorsPerTx);

      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 2n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 3n }),
        createValidator({ publicKey: "validator-3", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-4", withdrawableAmount: 0n }),
      ];
      mocks.getActiveValidatorsWithPendingWithdrawals.mockResolvedValueOnce(validators);

      await client.submitMaxAvailableWithdrawalRequests();

      expect(unstakeMock).toHaveBeenCalledTimes(2);

      const [, partialRequests] = unstakeMock.mock.calls[0];
      expect(partialRequests.pubkeys).toEqual([stringToHex("validator-1"), stringToHex("validator-2")]);
      expect(partialRequests.amountsGwei).toEqual([2n, 3n]);

      const [, exitRequests] = unstakeMock.mock.calls[1];
      expect(exitRequests.pubkeys).toEqual([stringToHex("validator-3")]);
      expect(exitRequests.amountsGwei).toEqual([0n]);

      expect(mocks.addValidatorPartialUnstakeAmount).toHaveBeenCalledTimes(2);
      expect(mocks.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(1, stringToHex("validator-1"), 2);
      expect(mocks.addValidatorPartialUnstakeAmount).toHaveBeenNthCalledWith(2, stringToHex("validator-2"), 3);

      expect(mocks.incrementValidatorExit).toHaveBeenCalledTimes(1);
      expect(mocks.incrementValidatorExit).toHaveBeenCalledWith(stringToHex("validator-3"));
    });
  });

  describe("_submitValidatorExits (private)", () => {
    it("returns immediately when no withdrawal slots remain", async () => {
      const { client, unstakeMock, mocks } = setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 0n }),
      ];

      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, 0);

      expect(unstakeMock).not.toHaveBeenCalled();
      expect(mocks.incrementValidatorExit).not.toHaveBeenCalled();
    });

    it("returns without unstaking when no validators qualify for exits", async () => {
      const { client, unstakeMock, mocks } = setupClient();
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 1n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 2n }),
      ];

      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, 2);

      expect(unstakeMock).not.toHaveBeenCalled();
      expect(mocks.incrementValidatorExit).not.toHaveBeenCalled();
    });

    it("stops adding exits when reaching the maximum per-transaction limit even with remaining capacity", async () => {
      const maxValidatorsPerTx = 3;
      const { client, unstakeMock, mocks } = setupClient(maxValidatorsPerTx);
      const validators = [
        createValidator({ publicKey: "validator-1", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-2", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-3", withdrawableAmount: 0n }),
        createValidator({ publicKey: "validator-4", withdrawableAmount: 0n }),
      ];

      await (
        client as unknown as {
          _submitValidatorExits(
            list: ValidatorBalanceWithPendingWithdrawal[],
            remainingWithdrawals: number,
          ): Promise<void>;
        }
      )._submitValidatorExits(validators, maxValidatorsPerTx + 1);

      expect(unstakeMock).toHaveBeenCalledTimes(1);
      const [, requests] = unstakeMock.mock.calls[0];
      expect(requests.pubkeys).toEqual([
        stringToHex("validator-1"),
        stringToHex("validator-2"),
        stringToHex("validator-3"),
      ]);
      expect(mocks.incrementValidatorExit).toHaveBeenCalledTimes(3);
      expect(mocks.incrementValidatorExit).toHaveBeenNthCalledWith(1, stringToHex("validator-1"));
      expect(mocks.incrementValidatorExit).toHaveBeenNthCalledWith(2, stringToHex("validator-2"));
      expect(mocks.incrementValidatorExit).toHaveBeenNthCalledWith(3, stringToHex("validator-3"));
    });
  });
});
