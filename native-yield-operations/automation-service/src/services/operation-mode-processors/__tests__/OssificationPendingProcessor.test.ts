import { jest } from "@jest/globals";
import type { TransactionReceipt, Address } from "viem";
import { ResultAsync } from "neverthrow";

import type { ILogger } from "@consensys/linea-shared-utils";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IOperationModeMetricsRecorder } from "../../../core/metrics/IOperationModeMetricsRecorder.js";
import type { IYieldManager } from "../../../core/clients/contracts/IYieldManager.js";
import type { ILazyOracle } from "../../../core/clients/contracts/ILazyOracle.js";
import type { ILidoAccountingReportClient } from "../../../core/clients/ILidoAccountingReportClient.js";
import type { IBeaconChainStakingClient } from "../../../core/clients/IBeaconChainStakingClient.js";
import type { IVaultHub } from "../../../core/clients/contracts/IVaultHub.js";
import { OperationMode } from "../../../core/enums/OperationModeEnums.js";
import { OperationTrigger } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { OssificationPendingProcessor } from "../OssificationPendingProcessor.js";
import { createLoggerMock, createMetricsUpdaterMock } from "../../../__tests__/helpers/index.js";

jest.mock("@consensys/linea-shared-utils", () => {
  const actual = jest.requireActual("@consensys/linea-shared-utils") as typeof import("@consensys/linea-shared-utils");
  return {
    ...actual,
    attempt: jest.fn(),
    msToSeconds: jest.fn(),
  };
});

import { attempt, msToSeconds } from "@consensys/linea-shared-utils";

// Semantic constants
const YIELD_PROVIDER_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
const VAULT_ADDRESS = "0x2222222222222222222222222222222222222222" as Address;
const TRANSACTION_HASH = "0xhash" as `0x${string}`;
const REPORT_CID = "cid";
const TREE_ROOT = "0x" as `0x${string}`;
const START_TIME_MS = 1000;
const END_TIME_MS = 1600;
const EXPECTED_DURATION_SECONDS = 0.6;
const OPERATION_DURATION_MS = 600;

const createMetricsRecorderMock = (): jest.Mocked<IOperationModeMetricsRecorder> =>
  ({
    recordSafeWithdrawalMetrics: jest.fn(),
    recordProgressOssificationMetrics: jest.fn(),
    recordReportYieldMetrics: jest.fn(),
    recordTransferFundsMetrics: jest.fn(),
  }) as unknown as jest.Mocked<IOperationModeMetricsRecorder>;

const createYieldManagerMock = (): jest.Mocked<IYieldManager<TransactionReceipt>> =>
  ({
    progressPendingOssification: jest.fn(),
    safeMaxAddToWithdrawalReserve: jest.fn(),
    isOssified: jest.fn(),
    getLidoStakingVaultAddress: jest.fn(),
  }) as unknown as jest.Mocked<IYieldManager<TransactionReceipt>>;

const createLazyOracleMock = (): jest.Mocked<ILazyOracle<TransactionReceipt>> =>
  ({
    waitForVaultsReportDataUpdatedEvent: jest.fn(),
  }) as unknown as jest.Mocked<ILazyOracle<TransactionReceipt>>;

const createLidoReportClientMock = (): jest.Mocked<ILidoAccountingReportClient> =>
  ({
    getLatestSubmitVaultReportParams: jest.fn(),
    submitLatestVaultReport: jest.fn(),
  }) as unknown as jest.Mocked<ILidoAccountingReportClient>;

const createBeaconClientMock = (): jest.Mocked<IBeaconChainStakingClient> =>
  ({
    submitMaxAvailableWithdrawalRequests: jest.fn(),
  }) as unknown as jest.Mocked<IBeaconChainStakingClient>;

const createVaultHubClientMock = (): jest.Mocked<IVaultHub<TransactionReceipt>> =>
  ({
    isReportFresh: jest.fn(),
    isVaultConnected: jest.fn(),
  }) as unknown as jest.Mocked<IVaultHub<TransactionReceipt>>;

const createTransactionReceipt = (): TransactionReceipt => ({
  transactionHash: TRANSACTION_HASH,
} as unknown as TransactionReceipt);

const createVaultReportEvent = () => ({
  result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
  report: {
    timestamp: 0n,
    refSlot: 0n,
    treeRoot: TREE_ROOT,
    reportCid: REPORT_CID,
  },
  txHash: TRANSACTION_HASH,
});

const createVaultReportParams = () => ({
  vault: VAULT_ADDRESS,
  totalValue: 0n,
  cumulativeLidoFees: 0n,
  liabilityShares: 0n,
  maxLiabilityShares: 0n,
  slashingReserve: 0n,
  proof: [],
});

describe("OssificationPendingProcessor", () => {
  let logger: jest.Mocked<ILogger>;
  let metricsUpdater: jest.Mocked<INativeYieldAutomationMetricsUpdater>;
  let metricsRecorder: jest.Mocked<IOperationModeMetricsRecorder>;
  let yieldManager: jest.Mocked<IYieldManager<TransactionReceipt>>;
  let lazyOracle: jest.Mocked<ILazyOracle<TransactionReceipt>>;
  let lidoReportClient: jest.Mocked<ILidoAccountingReportClient>;
  let beaconClient: jest.Mocked<IBeaconChainStakingClient>;
  let vaultHubClient: jest.Mocked<IVaultHub<TransactionReceipt>>;
  const attemptMock = attempt as jest.MockedFunction<typeof attempt>;
  const msToSecondsMock = msToSeconds as jest.MockedFunction<typeof msToSeconds>;

  const createProcessor = (shouldSubmitVaultReport: boolean = true) =>
    new OssificationPendingProcessor(
      logger,
      metricsUpdater,
      metricsRecorder,
      yieldManager,
      lazyOracle,
      lidoReportClient,
      beaconClient,
      vaultHubClient,
      YIELD_PROVIDER_ADDRESS,
      shouldSubmitVaultReport,
    );

  beforeEach(() => {
    jest.clearAllMocks();

    logger = createLoggerMock() as unknown as jest.Mocked<ILogger>;
    metricsUpdater = createMetricsUpdaterMock() as unknown as jest.Mocked<INativeYieldAutomationMetricsUpdater>;
    metricsRecorder = createMetricsRecorderMock();
    yieldManager = createYieldManagerMock();
    lazyOracle = createLazyOracleMock();
    lidoReportClient = createLidoReportClientMock();
    beaconClient = createBeaconClientMock();
    vaultHubClient = createVaultHubClientMock();

    lazyOracle.waitForVaultsReportDataUpdatedEvent.mockResolvedValue(createVaultReportEvent());
    yieldManager.getLidoStakingVaultAddress.mockResolvedValue(VAULT_ADDRESS);
    vaultHubClient.isVaultConnected.mockResolvedValue(true);
    vaultHubClient.isReportFresh.mockResolvedValue(false);
    lidoReportClient.getLatestSubmitVaultReportParams.mockResolvedValue(createVaultReportParams());
    lidoReportClient.submitLatestVaultReport.mockResolvedValue(undefined);
    yieldManager.progressPendingOssification.mockResolvedValue(createTransactionReceipt());
    yieldManager.isOssified.mockResolvedValue(false);
    yieldManager.safeMaxAddToWithdrawalReserve.mockResolvedValue(undefined as unknown as TransactionReceipt);
    msToSecondsMock.mockImplementation((ms: number) => ms / 1_000);
    attemptMock.mockImplementation(((logger: ILogger, fn: () => unknown | Promise<unknown>) =>
      ResultAsync.fromPromise(
        Promise.resolve().then(() => fn()),
        (error) => error as Error,
      )) as typeof attempt);
  });

  describe("process", () => {
    it("waits for trigger event before processing", async () => {
      // Arrange
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(lazyOracle.waitForVaultsReportDataUpdatedEvent).toHaveBeenCalledTimes(1);
    });

    it("submits max available withdrawal requests from beacon chain", async () => {
      // Arrange
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(beaconClient.submitMaxAvailableWithdrawalRequests).toHaveBeenCalledTimes(1);
    });

    it("retrieves lido staking vault address for yield provider", async () => {
      // Arrange
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(yieldManager.getLidoStakingVaultAddress).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
    });

    it("submits vault report when vault is connected and report is stale", async () => {
      // Arrange
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(lidoReportClient.getLatestSubmitVaultReportParams).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsUpdater.incrementLidoVaultAccountingReport).toHaveBeenCalledWith(VAULT_ADDRESS);
    });

    it("progresses pending ossification for yield provider", async () => {
      // Arrange
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(yieldManager.progressPendingOssification).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
    });

    it("records ossification metrics after successful progress", async () => {
      // Arrange
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(metricsRecorder.recordProgressOssificationMetrics).toHaveBeenCalledWith(
        YIELD_PROVIDER_ADDRESS,
        expect.anything(),
      );
    });

    it("checks if yield provider is ossified after progress", async () => {
      // Arrange
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(yieldManager.isOssified).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
    });

    it("records operation mode duration metrics", async () => {
      // Arrange
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(START_TIME_MS).mockReturnValueOnce(END_TIME_MS);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(msToSeconds).toHaveBeenCalledWith(OPERATION_DURATION_MS);
      expect(metricsUpdater.recordOperationModeDuration).toHaveBeenCalledWith(
        OperationMode.OSSIFICATION_PENDING_MODE,
        EXPECTED_DURATION_SECONDS,
      );

      performanceSpy.mockRestore();
    });
  });

  describe("vault report submission", () => {
    it("skips vault report when shouldSubmitVaultReport is false", async () => {
      // Arrange
      const processor = createProcessor(false);

      // Act
      await processor.process();

      // Assert
      expect(vaultHubClient.isReportFresh).not.toHaveBeenCalled();
      expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
      expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith(
        "_process - Skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)",
      );
    });

    it("skips vault report when vault is not connected", async () => {
      // Arrange
      vaultHubClient.isVaultConnected.mockResolvedValue(false);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).not.toHaveBeenCalled();
      expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
      expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith("_process - Skipping vault report submission (vault is not connected)");
    });

    it("skips vault report when report is fresh", async () => {
      // Arrange
      vaultHubClient.isReportFresh.mockResolvedValue(true);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
      expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith("_process - Skipping vault report submission (report is fresh)");
    });

    it("proceeds with submission when isReportFresh check fails", async () => {
      // Arrange
      vaultHubClient.isReportFresh.mockRejectedValue(new Error("check failed"));
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining("Failed to check if report is fresh, proceeding with submission attempt"),
      );
      expect(lidoReportClient.getLatestSubmitVaultReportParams).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(VAULT_ADDRESS);
    });

    it("continues processing when vault report submission fails", async () => {
      // Arrange
      lidoReportClient.submitLatestVaultReport.mockRejectedValue(new Error("submission failed"));
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(yieldManager.progressPendingOssification).toHaveBeenCalled();
    });
  });

  describe("ossification progress", () => {
    it("stops processing when progressPendingOssification fails", async () => {
      // Arrange
      const error = new Error("progress failed");
      yieldManager.progressPendingOssification.mockRejectedValue(error);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(logger.error).toHaveBeenCalledWith("_process - progressPendingOssification failed, stopping processing", {
        error: expect.any(Error),
      });
      expect(metricsRecorder.recordProgressOssificationMetrics).not.toHaveBeenCalled();
      expect(yieldManager.safeMaxAddToWithdrawalReserve).not.toHaveBeenCalled();
    });

    it("does not perform safe withdrawal when ossification is incomplete", async () => {
      // Arrange
      yieldManager.isOssified.mockResolvedValue(false);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(yieldManager.isOssified).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
      expect(metricsRecorder.recordSafeWithdrawalMetrics).not.toHaveBeenCalled();
    });

    it("performs safe withdrawal when ossification completes", async () => {
      // Arrange
      yieldManager.isOssified.mockResolvedValue(true);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(yieldManager.safeMaxAddToWithdrawalReserve).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS, expect.anything());
    });
  });

  describe("error handling", () => {
    it("tolerates failure of submitMaxAvailableWithdrawalRequests", async () => {
      // Arrange
      const failure = new Error("unstake failed");
      beaconClient.submitMaxAvailableWithdrawalRequests.mockRejectedValue(failure);
      yieldManager.isOssified.mockResolvedValue(true);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(beaconClient.submitMaxAvailableWithdrawalRequests).toHaveBeenCalledTimes(1);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(lidoReportClient.getLatestSubmitVaultReportParams).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsRecorder.recordProgressOssificationMetrics).toHaveBeenCalledWith(
        YIELD_PROVIDER_ADDRESS,
        expect.anything(),
      );
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS, expect.anything());
    });
  });
});
