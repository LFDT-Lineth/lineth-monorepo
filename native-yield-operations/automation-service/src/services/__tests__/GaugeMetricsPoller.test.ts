import type { ILogger, IBeaconNodeAPIClient } from "@consensys/linea-shared-utils";
import type { IValidatorDataClient } from "../../core/clients/IValidatorDataClient.js";
import type { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import type { IVaultHub } from "../../core/clients/contracts/IVaultHub.js";
import type { ISTETH } from "../../core/clients/contracts/ISTETH.js";
import type { ExitedValidator, ExitingValidator, ValidatorBalanceWithPendingWithdrawal, ValidatorBalance } from "../../core/entities/Validator.js";
import type { TransactionReceipt, Address } from "viem";
import { ONE_GWEI } from "@consensys/linea-shared-utils";
import { GaugeMetricsPoller } from "../GaugeMetricsPoller.js";
import { YieldProviderData } from "../../core/clients/contracts/IYieldManager.js";
import { DashboardContractClient } from "../../clients/contracts/DashboardContractClient.js";
import { createLoggerMock } from "../../__tests__/helpers/index.js";

jest.mock("../../clients/contracts/DashboardContractClient.js", () => ({
  DashboardContractClient: {
    getOrCreate: jest.fn(),
    initialize: jest.fn(),
  },
}));

// Semantic constants
const YIELD_PROVIDER_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
const VAULT_ADDRESS = "0x2222222222222222222222222222222222222222" as Address;
const DASHBOARD_ADDRESS = "0x3333333333333333333333333333333333333333" as Address;
const ZERO_ADDRESS = "0x0000000000000000000000000000000000000000" as Address;
const POLL_INTERVAL_MS = 5000;
const VAULT_WITHDRAWAL_CREDENTIALS = "0x0200000000000000000000002222222222222222222222222222222222222222";

// Factory functions for test data
const createValidatorBalance = (overrides?: Partial<ValidatorBalance>): ValidatorBalance => ({
  balance: 32n * ONE_GWEI,
  effectiveBalance: 32n * ONE_GWEI,
  publicKey: "validator-pubkey",
  validatorIndex: 1n,
  activationEpoch: 0,
  ...overrides,
});

const createValidatorBalanceWithPendingWithdrawal = (
  overrides?: Partial<ValidatorBalanceWithPendingWithdrawal>,
): ValidatorBalanceWithPendingWithdrawal => ({
  balance: 32n * ONE_GWEI,
  effectiveBalance: 32n * ONE_GWEI,
  publicKey: "validator-pubkey",
  validatorIndex: 1n,
  activationEpoch: 0,
  pendingWithdrawalAmount: 0n,
  withdrawableAmount: 0n,
  ...overrides,
});

const createExitingValidator = (overrides?: Partial<ExitingValidator>): ExitingValidator => ({
  balance: 32n * ONE_GWEI,
  effectiveBalance: 32n * ONE_GWEI,
  publicKey: "0xvalidator",
  validatorIndex: 1n,
  exitEpoch: 100,
  exitDate: new Date("2024-01-15T10:30:00Z"),
  slashed: false,
  ...overrides,
});

const createExitedValidator = (overrides?: Partial<ExitedValidator>): ExitedValidator => ({
  balance: 32n * ONE_GWEI,
  publicKey: "0xvalidator",
  validatorIndex: 1n,
  slashed: false,
  withdrawableEpoch: 100,
  ...overrides,
});

const createYieldProviderData = (overrides?: Partial<YieldProviderData>): YieldProviderData => ({
  yieldProviderVendor: 0,
  isStakingPaused: false,
  isOssificationInitiated: false,
  isOssified: false,
  primaryEntrypoint: ZERO_ADDRESS,
  ossifiedEntrypoint: ZERO_ADDRESS,
  yieldProviderIndex: 0n,
  userFunds: 0n,
  yieldReportedCumulative: 0n,
  lstLiabilityPrincipal: 0n,
  lastReportedNegativeYield: 0n,
  ...overrides,
});

const createPendingWithdrawal = (validator_index: number, amount: bigint, withdrawable_epoch: number) => ({
  validator_index,
  amount,
  withdrawable_epoch,
});

const createAggregatedWithdrawal = (
  validator_index: number,
  withdrawable_epoch: number,
  amount: bigint,
  pubkey: string,
) => ({
  validator_index,
  withdrawable_epoch,
  amount,
  pubkey,
});

const createPendingDeposit = (
  pubkey: string,
  withdrawal_credentials: string,
  amount: number,
  slot: number,
) => ({
  pubkey,
  withdrawal_credentials,
  amount,
  signature: "0xsignature",
  slot,
});

describe("GaugeMetricsPoller", () => {
  let logger: jest.Mocked<ILogger>;
  let validatorDataClient: jest.Mocked<IValidatorDataClient>;
  let metricsUpdater: jest.Mocked<INativeYieldAutomationMetricsUpdater>;
  let yieldManagerContractClient: jest.Mocked<IYieldManager<TransactionReceipt>>;
  let vaultHubContractClient: jest.Mocked<IVaultHub<TransactionReceipt>>;
  let beaconNodeApiClient: jest.Mocked<IBeaconNodeAPIClient>;
  let stethContractClient: jest.Mocked<ISTETH>;
  let dashboardClientInstance: jest.Mocked<DashboardContractClient>;
  let poller: GaugeMetricsPoller;

  beforeEach(() => {
    jest.useFakeTimers();
    logger = createLoggerMock();

    validatorDataClient = {
      getActiveValidators: jest.fn().mockResolvedValue([]),
      getExitingValidators: jest.fn().mockResolvedValue([]),
      getExitedValidators: jest.fn().mockResolvedValue([]),
      getValidatorsForWithdrawalRequestsAscending: jest.fn().mockResolvedValue([]),
      joinValidatorsWithPendingWithdrawals: jest.fn().mockReturnValue([]),
      getTotalPendingPartialWithdrawalsWei: jest.fn().mockReturnValue(0n),
      getFilteredAndAggregatedPendingWithdrawals: jest.fn().mockReturnValue([]),
      getTotalValidatorBalanceGwei: jest.fn().mockReturnValue(undefined),
      getTotalBalanceOfExitingValidators: jest.fn().mockReturnValue(undefined),
      getTotalBalanceOfExitedValidators: jest.fn().mockReturnValue(undefined),
    } as unknown as jest.Mocked<IValidatorDataClient>;

    metricsUpdater = {
      setLastTotalPendingPartialWithdrawalsGwei: jest.fn(),
      setPendingPartialWithdrawalQueueAmountGwei: jest.fn(),
      setLastTotalValidatorBalanceGwei: jest.fn(),
      setYieldReportedCumulative: jest.fn(),
      setLstLiabilityPrincipalGwei: jest.fn(),
      setLastReportedNegativeYield: jest.fn(),
      setLidoLstLiabilityGwei: jest.fn(),
      setLastVaultReportTimestamp: jest.fn(),
      setPendingDepositQueueAmountGwei: jest.fn(),
      setLastTotalPendingDepositGwei: jest.fn(),
      setPendingExitQueueAmountGwei: jest.fn(),
      setLastTotalPendingExitGwei: jest.fn(),
      setPendingFullWithdrawalQueueAmountGwei: jest.fn(),
      setLastTotalPendingFullWithdrawalGwei: jest.fn(),
    } as unknown as jest.Mocked<INativeYieldAutomationMetricsUpdater>;

    yieldManagerContractClient = {
      getYieldProviderData: jest.fn().mockResolvedValue(createYieldProviderData()),
      getLidoStakingVaultAddress: jest.fn().mockResolvedValue(VAULT_ADDRESS),
    } as unknown as jest.Mocked<IYieldManager<TransactionReceipt>>;

    vaultHubContractClient = {
      getLatestVaultReportTimestamp: jest.fn().mockResolvedValue(0n),
    } as unknown as jest.Mocked<IVaultHub<TransactionReceipt>>;

    beaconNodeApiClient = {
      getPendingPartialWithdrawals: jest.fn().mockResolvedValue([]),
      getPendingDeposits: jest.fn().mockResolvedValue([]),
    } as unknown as jest.Mocked<IBeaconNodeAPIClient>;

    stethContractClient = {
      getPooledEthBySharesRoundUp: jest.fn().mockResolvedValue(0n),
    } as unknown as jest.Mocked<ISTETH>;

    dashboardClientInstance = {
      liabilityShares: jest.fn().mockResolvedValue(0n),
      getAddress: jest.fn(),
      getContract: jest.fn(),
    } as unknown as jest.Mocked<DashboardContractClient>;

    (DashboardContractClient.getOrCreate as jest.MockedFunction<typeof DashboardContractClient.getOrCreate>).mockReturnValue(
      dashboardClientInstance,
    );

    poller = new GaugeMetricsPoller(
      logger,
      validatorDataClient,
      metricsUpdater,
      yieldManagerContractClient,
      vaultHubContractClient,
      YIELD_PROVIDER_ADDRESS,
      beaconNodeApiClient,
      POLL_INTERVAL_MS,
      stethContractClient,
    );
  });

  afterEach(() => {
    poller.stop();
    jest.useRealTimers();
  });

  describe("poll", () => {
    describe("LastTotalPendingPartialWithdrawalsGwei gauge", () => {
      it("updates metric with total pending partial withdrawals", async () => {
        // Arrange
        const totalPendingGwei = 4;
        const allValidators = [
          createValidatorBalance({ publicKey: "validator-1", validatorIndex: 1n }),
          createValidatorBalance({ publicKey: "validator-2", validatorIndex: 2n }),
        ];
        const pendingWithdrawalsQueue = [
          createPendingWithdrawal(1, 3n * ONE_GWEI, 0),
          createPendingWithdrawal(2, 1n * ONE_GWEI, 0),
        ];
        const joinedValidators = [
          createValidatorBalanceWithPendingWithdrawal({
            publicKey: "validator-1",
            validatorIndex: 1n,
            pendingWithdrawalAmount: 3n * ONE_GWEI,
          }),
          createValidatorBalanceWithPendingWithdrawal({
            publicKey: "validator-2",
            validatorIndex: 2n,
            pendingWithdrawalAmount: 1n * ONE_GWEI,
          }),
        ];

        validatorDataClient.getActiveValidators.mockResolvedValue(allValidators);
        beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValue(pendingWithdrawalsQueue);
        validatorDataClient.joinValidatorsWithPendingWithdrawals.mockReturnValue(joinedValidators);
        validatorDataClient.getTotalPendingPartialWithdrawalsWei.mockReturnValue(
          BigInt(totalPendingGwei) * ONE_GWEI,
        );
        validatorDataClient.getFilteredAndAggregatedPendingWithdrawals.mockReturnValue([]);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.joinValidatorsWithPendingWithdrawals).toHaveBeenCalledWith(
          allValidators,
          pendingWithdrawalsQueue,
        );
        expect(validatorDataClient.getTotalPendingPartialWithdrawalsWei).toHaveBeenCalledWith(joinedValidators);
        expect(metricsUpdater.setLastTotalPendingPartialWithdrawalsGwei).toHaveBeenCalledWith(totalPendingGwei);
      });

      it("skips update when validator data is unavailable", async () => {
        // Arrange
        validatorDataClient.getActiveValidators.mockResolvedValue(undefined);
        validatorDataClient.joinValidatorsWithPendingWithdrawals.mockReturnValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalPendingPartialWithdrawalsWei).not.toHaveBeenCalled();
        expect(metricsUpdater.setLastTotalPendingPartialWithdrawalsGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping total pending partial withdrawals gauge update: validator data unavailable",
        );
      });
    });

    describe("LastTotalValidatorBalanceGwei gauge", () => {
      it("updates metric with total validator balance", async () => {
        // Arrange
        const totalBalanceGwei = 72n * ONE_GWEI;
        const allValidators = [
          createValidatorBalance({ balance: 32n * ONE_GWEI, publicKey: "validator-1", validatorIndex: 1n }),
          createValidatorBalance({ balance: 40n * ONE_GWEI, publicKey: "validator-2", validatorIndex: 2n }),
        ];

        validatorDataClient.getActiveValidators.mockResolvedValue(allValidators);
        validatorDataClient.getTotalValidatorBalanceGwei.mockReturnValue(totalBalanceGwei);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalValidatorBalanceGwei).toHaveBeenCalledWith(allValidators);
        expect(metricsUpdater.setLastTotalValidatorBalanceGwei).toHaveBeenCalledWith(Number(totalBalanceGwei));
      });

      it("skips update when validator list is undefined", async () => {
        // Arrange
        validatorDataClient.getActiveValidators.mockResolvedValue(undefined);
        validatorDataClient.getTotalValidatorBalanceGwei.mockReturnValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalValidatorBalanceGwei).toHaveBeenCalledWith(undefined);
        expect(metricsUpdater.setLastTotalValidatorBalanceGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping total validator balance gauge update: validator balance unavailable",
        );
      });

      it("skips update when balance calculation returns undefined", async () => {
        // Arrange
        validatorDataClient.getActiveValidators.mockResolvedValue([]);
        validatorDataClient.getTotalValidatorBalanceGwei.mockReturnValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalValidatorBalanceGwei).toHaveBeenCalledWith([]);
        expect(metricsUpdater.setLastTotalValidatorBalanceGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping total validator balance gauge update: validator balance unavailable",
        );
      });
    });

    describe("YieldReportedCumulative gauge", () => {
      it("updates metric with yield reported cumulative value", async () => {
        // Arrange
        const yieldReportedGwei = 1000;
        const yieldProviderData = createYieldProviderData({
          yieldReportedCumulative: BigInt(yieldReportedGwei) * ONE_GWEI,
        });

        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(yieldProviderData);

        // Act
        await poller.poll();

        // Assert
        expect(yieldManagerContractClient.getYieldProviderData).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(yieldManagerContractClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(metricsUpdater.setYieldReportedCumulative).toHaveBeenCalledWith(VAULT_ADDRESS, yieldReportedGwei);
      });
    });

    describe("LstLiabilityPrincipalGwei gauge", () => {
      it("updates metric with LST liability principal value", async () => {
        // Arrange
        const lstLiabilityGwei = 5000;
        const yieldProviderData = createYieldProviderData({
          lstLiabilityPrincipal: BigInt(lstLiabilityGwei) * ONE_GWEI,
        });

        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(yieldProviderData);

        // Act
        await poller.poll();

        // Assert
        expect(yieldManagerContractClient.getYieldProviderData).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(yieldManagerContractClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(metricsUpdater.setLstLiabilityPrincipalGwei).toHaveBeenCalledWith(VAULT_ADDRESS, lstLiabilityGwei);
      });
    });

    describe("LastReportedNegativeYield gauge", () => {
      it("updates metric with last reported negative yield value", async () => {
        // Arrange
        const lastNegativeYieldGwei = 3000;
        const yieldProviderData = createYieldProviderData({
          lastReportedNegativeYield: BigInt(lastNegativeYieldGwei) * ONE_GWEI,
        });

        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(yieldProviderData);

        // Act
        await poller.poll();

        // Assert
        expect(yieldManagerContractClient.getYieldProviderData).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(yieldManagerContractClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(metricsUpdater.setLastReportedNegativeYield).toHaveBeenCalledWith(
          VAULT_ADDRESS,
          lastNegativeYieldGwei,
        );
      });
    });

    describe("LidoLstLiabilityGwei gauge", () => {
      it("updates metric with Lido LST liability value", async () => {
        // Arrange
        const liabilityShares = 1000n;
        const pooledEthGwei = 2000;
        const yieldProviderData = createYieldProviderData({
          primaryEntrypoint: DASHBOARD_ADDRESS,
        });

        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(yieldProviderData);
        dashboardClientInstance.liabilityShares.mockResolvedValue(liabilityShares);
        stethContractClient.getPooledEthBySharesRoundUp.mockResolvedValue(BigInt(pooledEthGwei) * ONE_GWEI);

        // Act
        await poller.poll();

        // Assert
        expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(DASHBOARD_ADDRESS);
        expect(dashboardClientInstance.liabilityShares).toHaveBeenCalled();
        expect(stethContractClient.getPooledEthBySharesRoundUp).toHaveBeenCalledWith(liabilityShares);
        expect(metricsUpdater.setLidoLstLiabilityGwei).toHaveBeenCalledWith(VAULT_ADDRESS, pooledEthGwei);
      });

      it("skips update when getPooledEthBySharesRoundUp returns undefined", async () => {
        // Arrange
        const liabilityShares = 1000n;
        const yieldProviderData = createYieldProviderData({
          primaryEntrypoint: DASHBOARD_ADDRESS,
        });

        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(yieldProviderData);
        dashboardClientInstance.liabilityShares.mockResolvedValue(liabilityShares);
        stethContractClient.getPooledEthBySharesRoundUp.mockResolvedValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(DASHBOARD_ADDRESS);
        expect(dashboardClientInstance.liabilityShares).toHaveBeenCalled();
        expect(stethContractClient.getPooledEthBySharesRoundUp).toHaveBeenCalledWith(liabilityShares);
        expect(metricsUpdater.setLidoLstLiabilityGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          expect.stringContaining("Skipping Lido LST liability gauge update: getPooledEthBySharesRoundUp returned undefined"),
        );
      });

      it("logs error when liabilityShares fetch fails", async () => {
        // Arrange
        const error = new Error("Failed to fetch liability shares");
        const yieldProviderData = createYieldProviderData({
          primaryEntrypoint: DASHBOARD_ADDRESS,
        });

        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(yieldProviderData);
        dashboardClientInstance.liabilityShares.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(DASHBOARD_ADDRESS);
        expect(dashboardClientInstance.liabilityShares).toHaveBeenCalled();
        expect(stethContractClient.getPooledEthBySharesRoundUp).not.toHaveBeenCalled();
        expect(metricsUpdater.setLidoLstLiabilityGwei).not.toHaveBeenCalled();
        expect(logger.error).toHaveBeenCalledWith("Failed to update Lido LST liability gauge", {
          error,
          vault: VAULT_ADDRESS,
        });
      });
    });

    describe("LastVaultReportTimestamp gauge", () => {
      it("updates metric with latest vault report timestamp", async () => {
        // Arrange
        const expectedTimestamp = 1704067200n;

        vaultHubContractClient.getLatestVaultReportTimestamp.mockResolvedValue(expectedTimestamp);

        // Act
        await poller.poll();

        // Assert
        expect(yieldManagerContractClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(vaultHubContractClient.getLatestVaultReportTimestamp).toHaveBeenCalledWith(VAULT_ADDRESS);
        expect(metricsUpdater.setLastVaultReportTimestamp).toHaveBeenCalledWith(
          VAULT_ADDRESS,
          Number(expectedTimestamp),
        );
      });

      it("logs error when contract read fails", async () => {
        // Arrange
        const error = new Error("Contract read failed");

        vaultHubContractClient.getLatestVaultReportTimestamp.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(vaultHubContractClient.getLatestVaultReportTimestamp).toHaveBeenCalledWith(VAULT_ADDRESS);
        expect(metricsUpdater.setLastVaultReportTimestamp).not.toHaveBeenCalled();
        expect(logger.error).toHaveBeenCalledWith("Failed to update last vault report timestamp gauge metric", {
          error,
        });
      });
    });

    describe("PendingPartialWithdrawalQueueAmountGwei gauge", () => {
      it("updates metric for each aggregated withdrawal", async () => {
        // Arrange
        const allValidators = [
          createValidatorBalance({ publicKey: "validator-1-pubkey", validatorIndex: 1n }),
          createValidatorBalance({ publicKey: "validator-2-pubkey", validatorIndex: 2n }),
        ];
        const pendingWithdrawalsQueue = [
          createPendingWithdrawal(1, 3n * ONE_GWEI, 100),
          createPendingWithdrawal(1, 2n * ONE_GWEI, 100),
          createPendingWithdrawal(1, 5n * ONE_GWEI, 200),
          createPendingWithdrawal(2, 1n * ONE_GWEI, 100),
        ];
        const aggregatedWithdrawals = [
          createAggregatedWithdrawal(1, 100, 5n, "validator-1-pubkey"),
          createAggregatedWithdrawal(1, 200, 5n, "validator-1-pubkey"),
          createAggregatedWithdrawal(2, 100, 1n, "validator-2-pubkey"),
        ];

        validatorDataClient.getActiveValidators.mockResolvedValue(allValidators);
        beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValue(pendingWithdrawalsQueue);
        validatorDataClient.getFilteredAndAggregatedPendingWithdrawals.mockReturnValue(aggregatedWithdrawals);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getFilteredAndAggregatedPendingWithdrawals).toHaveBeenCalledWith(
          allValidators,
          pendingWithdrawalsQueue,
        );
        expect(metricsUpdater.setPendingPartialWithdrawalQueueAmountGwei).toHaveBeenCalledTimes(3);
        expect(metricsUpdater.setPendingPartialWithdrawalQueueAmountGwei).toHaveBeenNthCalledWith(
          1,
          "validator-1-pubkey",
          100,
          5,
        );
        expect(metricsUpdater.setPendingPartialWithdrawalQueueAmountGwei).toHaveBeenNthCalledWith(
          2,
          "validator-1-pubkey",
          200,
          5,
        );
        expect(metricsUpdater.setPendingPartialWithdrawalQueueAmountGwei).toHaveBeenNthCalledWith(
          3,
          "validator-2-pubkey",
          100,
          1,
        );
      });

      it("skips update when aggregated withdrawals are undefined", async () => {
        // Arrange
        validatorDataClient.getActiveValidators.mockResolvedValue([]);
        beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValue([]);
        validatorDataClient.getFilteredAndAggregatedPendingWithdrawals.mockReturnValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getFilteredAndAggregatedPendingWithdrawals).toHaveBeenCalled();
        expect(metricsUpdater.setPendingPartialWithdrawalQueueAmountGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping pending partial withdrawals queue gauge update: aggregated withdrawals unavailable",
        );
      });

      it("handles empty aggregated withdrawals array", async () => {
        // Arrange
        validatorDataClient.getActiveValidators.mockResolvedValue([]);
        beaconNodeApiClient.getPendingPartialWithdrawals.mockResolvedValue([]);
        validatorDataClient.getFilteredAndAggregatedPendingWithdrawals.mockReturnValue([]);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getFilteredAndAggregatedPendingWithdrawals).toHaveBeenCalled();
        expect(metricsUpdater.setPendingPartialWithdrawalQueueAmountGwei).not.toHaveBeenCalled();
      });
    });

    describe("PendingDepositQueueAmountGwei gauge", () => {
      it("updates metric for matching deposits", async () => {
        // Arrange
        const matchingDeposit = createPendingDeposit(
          "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
          VAULT_WITHDRAWAL_CREDENTIALS,
          32000000000,
          100,
        );
        const nonMatchingDeposit = createPendingDeposit(
          "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
          "0x0200000000000000000000003333333333333333333333333333333333333333",
          32000000000,
          101,
        );

        beaconNodeApiClient.getPendingDeposits.mockResolvedValue([matchingDeposit, nonMatchingDeposit]);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingDepositQueueAmountGwei).toHaveBeenCalledTimes(1);
        expect(metricsUpdater.setPendingDepositQueueAmountGwei).toHaveBeenCalledWith(
          matchingDeposit.pubkey,
          matchingDeposit.slot,
          matchingDeposit.amount,
        );
      });

      it("skips update when pending deposits are undefined", async () => {
        // Arrange
        beaconNodeApiClient.getPendingDeposits.mockResolvedValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingDepositQueueAmountGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping pending deposits queue gauge update: pending deposits data unavailable",
        );
      });

      it("handles empty pending deposits array", async () => {
        // Arrange
        beaconNodeApiClient.getPendingDeposits.mockResolvedValue([]);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingDepositQueueAmountGwei).not.toHaveBeenCalled();
      });
    });

    describe("LastTotalPendingDepositGwei gauge", () => {
      it("updates metric with sum of matching deposits", async () => {
        // Arrange
        const matchingDeposit1 = createPendingDeposit(
          "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
          VAULT_WITHDRAWAL_CREDENTIALS,
          32000000000,
          100,
        );
        const matchingDeposit2 = createPendingDeposit(
          "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
          VAULT_WITHDRAWAL_CREDENTIALS,
          16000000000,
          101,
        );
        const nonMatchingDeposit = createPendingDeposit(
          "0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba",
          "0x0200000000000000000000003333333333333333333333333333333333333333",
          32000000000,
          102,
        );

        beaconNodeApiClient.getPendingDeposits.mockResolvedValue([
          matchingDeposit1,
          matchingDeposit2,
          nonMatchingDeposit,
        ]);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setLastTotalPendingDepositGwei).toHaveBeenCalledTimes(1);
        expect(metricsUpdater.setLastTotalPendingDepositGwei).toHaveBeenCalledWith(48000000000);
      });

      it("handles empty matching deposits array", async () => {
        // Arrange
        const nonMatchingDeposit = createPendingDeposit(
          "0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba",
          "0x0200000000000000000000003333333333333333333333333333333333333333",
          32000000000,
          102,
        );

        beaconNodeApiClient.getPendingDeposits.mockResolvedValue([nonMatchingDeposit]);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setLastTotalPendingDepositGwei).toHaveBeenCalledTimes(1);
        expect(metricsUpdater.setLastTotalPendingDepositGwei).toHaveBeenCalledWith(0);
      });

      it("skips update when pending deposits are undefined", async () => {
        // Arrange
        beaconNodeApiClient.getPendingDeposits.mockResolvedValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setLastTotalPendingDepositGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping total pending deposits gauge update: pending deposits data unavailable",
        );
      });
    });

    describe("PendingExitQueueAmountGwei gauge", () => {
      it("updates metric for each exiting validator", async () => {
        // Arrange
        const exitingValidators = [
          createExitingValidator({ publicKey: "0xvalidator1", validatorIndex: 1n, exitEpoch: 100, slashed: false }),
          createExitingValidator({
            publicKey: "0xvalidator2",
            validatorIndex: 2n,
            exitEpoch: 150,
            balance: 40n * ONE_GWEI,
            slashed: true,
          }),
        ];

        validatorDataClient.getExitingValidators.mockResolvedValue(exitingValidators);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingExitQueueAmountGwei).toHaveBeenCalledTimes(2);
        expect(metricsUpdater.setPendingExitQueueAmountGwei).toHaveBeenCalledWith(
          "0xvalidator1",
          100,
          32000000000,
          false,
        );
        expect(metricsUpdater.setPendingExitQueueAmountGwei).toHaveBeenCalledWith(
          "0xvalidator2",
          150,
          40000000000,
          true,
        );
      });

      it("skips update when exiting validators are undefined", async () => {
        // Arrange
        validatorDataClient.getExitingValidators.mockResolvedValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingExitQueueAmountGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping pending exit queue gauge update: exiting validators data unavailable or empty",
        );
      });

      it("skips update when exiting validators array is empty", async () => {
        // Arrange
        validatorDataClient.getExitingValidators.mockResolvedValue([]);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingExitQueueAmountGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping pending exit queue gauge update: exiting validators data unavailable or empty",
        );
      });
    });

    describe("LastTotalPendingExitGwei gauge", () => {
      it("updates metric with total balance of exiting validators", async () => {
        // Arrange
        const totalBalanceGwei = 72n * ONE_GWEI;
        const exitingValidators = [
          createExitingValidator({ balance: 32n * ONE_GWEI, publicKey: "0xvalidator1", validatorIndex: 1n }),
          createExitingValidator({ balance: 40n * ONE_GWEI, publicKey: "0xvalidator2", validatorIndex: 2n }),
        ];

        validatorDataClient.getExitingValidators.mockResolvedValue(exitingValidators);
        validatorDataClient.getTotalBalanceOfExitingValidators.mockReturnValue(totalBalanceGwei);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalBalanceOfExitingValidators).toHaveBeenCalledWith(exitingValidators);
        expect(metricsUpdater.setLastTotalPendingExitGwei).toHaveBeenCalledWith(Number(totalBalanceGwei));
      });

      it("skips update when total balance is undefined", async () => {
        // Arrange
        const exitingValidators = [createExitingValidator()];

        validatorDataClient.getExitingValidators.mockResolvedValue(exitingValidators);
        validatorDataClient.getTotalBalanceOfExitingValidators.mockReturnValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setLastTotalPendingExitGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith("Skipping total pending exit gauge update: total balance unavailable");
      });

      it("sets metric to 0 for empty exiting validators array", async () => {
        // Arrange
        validatorDataClient.getExitingValidators.mockResolvedValue([]);
        validatorDataClient.getTotalBalanceOfExitingValidators.mockReturnValue(0n);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalBalanceOfExitingValidators).toHaveBeenCalledWith([]);
        expect(metricsUpdater.setLastTotalPendingExitGwei).toHaveBeenCalledWith(0);
      });
    });

    describe("PendingFullWithdrawalQueueAmountGwei gauge", () => {
      it("updates metric for each exited validator", async () => {
        // Arrange
        const exitedValidators = [
          createExitedValidator({
            publicKey: "0xvalidator1",
            validatorIndex: 1n,
            withdrawableEpoch: 100,
            slashed: false,
          }),
          createExitedValidator({
            publicKey: "0xvalidator2",
            validatorIndex: 2n,
            withdrawableEpoch: 150,
            balance: 40n * ONE_GWEI,
            slashed: true,
          }),
        ];

        validatorDataClient.getExitedValidators.mockResolvedValue(exitedValidators);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingFullWithdrawalQueueAmountGwei).toHaveBeenCalledTimes(2);
        expect(metricsUpdater.setPendingFullWithdrawalQueueAmountGwei).toHaveBeenCalledWith(
          "0xvalidator1",
          100,
          32000000000,
          false,
        );
        expect(metricsUpdater.setPendingFullWithdrawalQueueAmountGwei).toHaveBeenCalledWith(
          "0xvalidator2",
          150,
          40000000000,
          true,
        );
      });

      it("skips update when exited validators are undefined", async () => {
        // Arrange
        validatorDataClient.getExitedValidators.mockResolvedValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingFullWithdrawalQueueAmountGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping pending full withdrawal queue gauge update: exited validators data unavailable or empty",
        );
      });

      it("skips update when exited validators array is empty", async () => {
        // Arrange
        validatorDataClient.getExitedValidators.mockResolvedValue([]);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setPendingFullWithdrawalQueueAmountGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping pending full withdrawal queue gauge update: exited validators data unavailable or empty",
        );
      });
    });

    describe("LastTotalPendingFullWithdrawalGwei gauge", () => {
      it("updates metric with total balance of exited validators", async () => {
        // Arrange
        const totalBalanceGwei = 72n * ONE_GWEI;
        const exitedValidators = [
          createExitedValidator({ balance: 32n * ONE_GWEI, publicKey: "0xvalidator1", validatorIndex: 1n }),
          createExitedValidator({ balance: 40n * ONE_GWEI, publicKey: "0xvalidator2", validatorIndex: 2n }),
        ];

        validatorDataClient.getExitedValidators.mockResolvedValue(exitedValidators);
        validatorDataClient.getTotalBalanceOfExitedValidators.mockReturnValue(totalBalanceGwei);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalBalanceOfExitedValidators).toHaveBeenCalledWith(exitedValidators);
        expect(metricsUpdater.setLastTotalPendingFullWithdrawalGwei).toHaveBeenCalledWith(Number(totalBalanceGwei));
      });

      it("skips update when total balance is undefined", async () => {
        // Arrange
        const exitedValidators = [createExitedValidator()];

        validatorDataClient.getExitedValidators.mockResolvedValue(exitedValidators);
        validatorDataClient.getTotalBalanceOfExitedValidators.mockReturnValue(undefined);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setLastTotalPendingFullWithdrawalGwei).not.toHaveBeenCalled();
        expect(logger.warn).toHaveBeenCalledWith(
          "Skipping total pending full withdrawal gauge update: total balance unavailable",
        );
      });

      it("sets metric to 0 for empty exited validators array", async () => {
        // Arrange
        validatorDataClient.getExitedValidators.mockResolvedValue([]);
        validatorDataClient.getTotalBalanceOfExitedValidators.mockReturnValue(0n);

        // Act
        await poller.poll();

        // Assert
        expect(validatorDataClient.getTotalBalanceOfExitedValidators).toHaveBeenCalledWith([]);
        expect(metricsUpdater.setLastTotalPendingFullWithdrawalGwei).toHaveBeenCalledWith(0);
      });
    });

    describe("error handling", () => {
      it("handles active validators fetch failure", async () => {
        // Arrange
        const error = new Error("Validator data fetch failed");

        validatorDataClient.getActiveValidators.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(logger.error).toHaveBeenCalledWith("Failed to fetch active validators", { error });
        expect(metricsUpdater.setLastTotalValidatorBalanceGwei).not.toHaveBeenCalled();
      });

      it("handles exiting validators fetch failure", async () => {
        // Arrange
        const error = new Error("Exiting validators fetch failed");

        validatorDataClient.getExitingValidators.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(logger.error).toHaveBeenCalledWith("Failed to fetch exiting validators", { error });
      });

      it("handles exited validators fetch failure", async () => {
        // Arrange
        const error = new Error("Exited validators fetch failed");

        validatorDataClient.getExitedValidators.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(logger.error).toHaveBeenCalledWith("Failed to fetch exited validators", { error });
      });

      it("handles pending partial withdrawals fetch failure", async () => {
        // Arrange
        const error = new Error("Failed to fetch pending partial withdrawals");

        beaconNodeApiClient.getPendingPartialWithdrawals.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(logger.error).toHaveBeenCalledWith("Failed to fetch pending partial withdrawals", { error });
      });

      it("handles pending deposits fetch failure", async () => {
        // Arrange
        const error = new Error("Failed to fetch pending deposits");

        beaconNodeApiClient.getPendingDeposits.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(logger.error).toHaveBeenCalledWith("Failed to fetch pending deposits", { error });
        expect(metricsUpdater.setPendingDepositQueueAmountGwei).not.toHaveBeenCalled();
        expect(metricsUpdater.setLastTotalPendingDepositGwei).not.toHaveBeenCalled();
      });

      it("handles yield provider data fetch failure", async () => {
        // Arrange
        const error = new Error("Contract read failed");

        yieldManagerContractClient.getYieldProviderData.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(logger.error).toHaveBeenCalledWith("Failed to fetch yield provider data", { error });
        expect(metricsUpdater.setYieldReportedCumulative).not.toHaveBeenCalled();
        expect(metricsUpdater.setLstLiabilityPrincipalGwei).not.toHaveBeenCalled();
      });

      it("handles vault address fetch failure", async () => {
        // Arrange
        const error = new Error("Vault address fetch failed");

        yieldManagerContractClient.getLidoStakingVaultAddress.mockRejectedValue(error);

        // Act
        await poller.poll();

        // Assert
        expect(yieldManagerContractClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(logger.error).toHaveBeenCalledWith("Failed to fetch vault address, skipping vault-dependent metrics", {
          error,
        });
        expect(metricsUpdater.setYieldReportedCumulative).not.toHaveBeenCalled();
        expect(metricsUpdater.setLstLiabilityPrincipalGwei).not.toHaveBeenCalled();
        expect(metricsUpdater.setLastVaultReportTimestamp).not.toHaveBeenCalled();
      });
    });

    describe("conversion", () => {
      it("converts wei to gwei correctly", async () => {
        // Arrange
        const pendingWithdrawalsWei = 1500000000n;
        const yieldReportedWei = 2500000000n;
        const joinedValidators = [createValidatorBalanceWithPendingWithdrawal()];

        validatorDataClient.joinValidatorsWithPendingWithdrawals.mockReturnValue(joinedValidators);
        validatorDataClient.getTotalPendingPartialWithdrawalsWei.mockReturnValue(pendingWithdrawalsWei);
        validatorDataClient.getFilteredAndAggregatedPendingWithdrawals.mockReturnValue([]);
        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(
          createYieldProviderData({
            yieldReportedCumulative: yieldReportedWei,
          }),
        );

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setLastTotalPendingPartialWithdrawalsGwei).toHaveBeenCalledWith(1);
        expect(metricsUpdater.setYieldReportedCumulative).toHaveBeenCalledWith(VAULT_ADDRESS, 2);
      });

      it("converts timestamp bigint to number", async () => {
        // Arrange
        const expectedTimestamp = 1704067200n;

        vaultHubContractClient.getLatestVaultReportTimestamp.mockResolvedValue(expectedTimestamp);

        // Act
        await poller.poll();

        // Assert
        expect(metricsUpdater.setLastVaultReportTimestamp).toHaveBeenCalledWith(
          VAULT_ADDRESS,
          Number(expectedTimestamp),
        );
      });
    });

    describe("integration", () => {
      it("fetches vault address only once per poll", async () => {
        // Arrange
        const yieldReportedGwei = 500;
        const lstLiabilityGwei = 3000;
        const yieldProviderData = createYieldProviderData({
          yieldReportedCumulative: BigInt(yieldReportedGwei) * ONE_GWEI,
          lstLiabilityPrincipal: BigInt(lstLiabilityGwei) * ONE_GWEI,
        });

        yieldManagerContractClient.getYieldProviderData.mockResolvedValue(yieldProviderData);

        // Act
        await poller.poll();

        // Assert
        expect(yieldManagerContractClient.getLidoStakingVaultAddress).toHaveBeenCalledTimes(1);
        expect(yieldManagerContractClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
        expect(metricsUpdater.setYieldReportedCumulative).toHaveBeenCalledWith(VAULT_ADDRESS, yieldReportedGwei);
        expect(metricsUpdater.setLstLiabilityPrincipalGwei).toHaveBeenCalledWith(VAULT_ADDRESS, lstLiabilityGwei);
      });
    });
  });

  describe("start and stop", () => {
    it("starts polling at configured interval", async () => {
      // Arrange
      const pollSpy = jest.spyOn(poller, "poll");

      // Act
      poller.start();

      // Assert - initial poll
      expect(logger.info).toHaveBeenCalledWith("Starting gauge metrics polling loop");
      expect(pollSpy).toHaveBeenCalledTimes(1);

      // Advance timer and check subsequent poll
      await jest.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
      expect(pollSpy).toHaveBeenCalledTimes(2);
    });

    it("stops polling when stop is called", async () => {
      // Arrange
      const pollSpy = jest.spyOn(poller, "poll");

      // Act
      poller.start();
      poller.stop();
      await jest.advanceTimersByTimeAsync(POLL_INTERVAL_MS);

      // Assert - only the initial poll should have happened
      expect(logger.info).toHaveBeenCalledWith("Stopped gauge metrics polling loop");
      expect(pollSpy).toHaveBeenCalledTimes(1);
    });

    it("does not start multiple loops if already running", async () => {
      // Arrange
      poller.start();
      await jest.advanceTimersByTimeAsync(10);

      // Act
      await poller.start();

      // Assert
      expect(logger.debug).toHaveBeenCalledWith("GaugeMetricsPoller.start() - already running, skipping");
    });

    it("does not stop if not running", () => {
      // Arrange - poller not started

      // Act
      poller.stop();

      // Assert
      expect(logger.debug).toHaveBeenCalledWith("GaugeMetricsPoller.stop() - not running, skipping");
    });
  });
});
