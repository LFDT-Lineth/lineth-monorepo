import { ILogger } from "@consensys/linea-shared-utils";
import { jest, describe, it, expect, beforeEach } from "@jest/globals";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IOperationModeMetricsRecorder } from "../../../core/metrics/IOperationModeMetricsRecorder.js";
import type { IYieldManager } from "../../../core/clients/contracts/IYieldManager.js";
import type { TransactionReceipt, Address } from "viem";
import type { IBeaconChainStakingClient } from "../../../core/clients/IBeaconChainStakingClient.js";
import { OperationMode } from "../../../core/enums/OperationModeEnums.js";
import { OssificationCompleteProcessor } from "../OssificationCompleteProcessor.js";
import { ResultAsync } from "neverthrow";
import { createLoggerMock, createMetricsUpdaterMock } from "../../../__tests__/helpers/index.js";

jest.mock("@consensys/linea-shared-utils", () => {
  const actual = jest.requireActual("@consensys/linea-shared-utils") as typeof import("@consensys/linea-shared-utils");
  return {
    ...actual,
    wait: jest.fn(),
    attempt: jest.fn(),
    msToSeconds: jest.fn(),
  };
});

import { wait, attempt, msToSeconds } from "@consensys/linea-shared-utils";

const YIELD_PROVIDER_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
const MAX_INACTION_MS = 5_000;
const PERFORMANCE_START = 100;
const PERFORMANCE_END = 340;
const EXPECTED_DURATION_SECONDS = 0.24;

const createMetricsRecorderMock = () => ({
  recordSafeWithdrawalMetrics: jest.fn(),
  recordProgressOssificationMetrics: jest.fn(),
  recordReportYieldMetrics: jest.fn(),
  recordTransferFundsMetrics: jest.fn(),
});

const createYieldManagerMock = () => ({
  safeMaxAddToWithdrawalReserve: jest.fn(),
});

const createBeaconClientMock = () => ({
  submitMaxAvailableWithdrawalRequests: jest.fn(),
});

describe("OssificationCompleteProcessor", () => {
  let processor: OssificationCompleteProcessor;
  let logger: jest.Mocked<ILogger>;
  let metricsUpdater: jest.Mocked<INativeYieldAutomationMetricsUpdater>;
  let metricsRecorder: jest.Mocked<IOperationModeMetricsRecorder>;
  let yieldManager: jest.Mocked<IYieldManager<TransactionReceipt>>;
  let beaconClient: jest.Mocked<IBeaconChainStakingClient>;

  const waitMock = wait as jest.MockedFunction<typeof wait>;
  const attemptMock = attempt as jest.MockedFunction<typeof attempt>;
  const msToSecondsMock = msToSeconds as jest.MockedFunction<typeof msToSeconds>;

  beforeEach(() => {
    jest.clearAllMocks();
    logger = createLoggerMock() as unknown as jest.Mocked<ILogger>;
    metricsUpdater = createMetricsUpdaterMock() as unknown as jest.Mocked<INativeYieldAutomationMetricsUpdater>;
    metricsRecorder = createMetricsRecorderMock() as unknown as jest.Mocked<IOperationModeMetricsRecorder>;
    yieldManager = createYieldManagerMock() as unknown as jest.Mocked<IYieldManager<TransactionReceipt>>;
    beaconClient = createBeaconClientMock() as unknown as jest.Mocked<IBeaconChainStakingClient>;

    waitMock.mockResolvedValue(undefined);
    msToSecondsMock.mockImplementation((ms: number) => ms / 1_000);
    yieldManager.safeMaxAddToWithdrawalReserve.mockResolvedValue(undefined as unknown as TransactionReceipt);
    attemptMock.mockImplementation(((logger: ILogger, fn: () => unknown | Promise<unknown>) =>
      ResultAsync.fromPromise(
        Promise.resolve().then(() => fn()),
        (error) => error as Error,
      )) as typeof attempt);

    processor = new OssificationCompleteProcessor(
      logger,
      metricsUpdater,
      metricsRecorder,
      yieldManager,
      beaconClient,
      MAX_INACTION_MS,
      YIELD_PROVIDER_ADDRESS,
    );
  });

  describe("process", () => {
    it("waits for configured inaction period before executing", async () => {
      // Arrange
      // (processor configured in beforeEach)

      // Act
      await processor.process();

      // Assert
      expect(logger.info).toHaveBeenCalledWith(`Waiting ${MAX_INACTION_MS}ms before executing actions`);
      expect(waitMock).toHaveBeenCalledWith(MAX_INACTION_MS);
    });

    it("performs max withdrawal from yield provider", async () => {
      // Arrange
      // (processor configured in beforeEach)

      // Act
      await processor.process();

      // Assert
      expect(attemptMock).toHaveBeenCalledWith(
        logger,
        expect.any(Function),
        "_process - safeMaxAddToWithdrawalReserve failed (tolerated)",
      );
      expect(yieldManager.safeMaxAddToWithdrawalReserve).toHaveBeenCalledWith(YIELD_PROVIDER_ADDRESS);
    });

    it("records successful withdrawal metrics", async () => {
      // Arrange
      // (processor configured in beforeEach)

      // Act
      await processor.process();

      // Assert
      const recordedResult = metricsRecorder.recordSafeWithdrawalMetrics.mock.calls[0]?.[1];
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledWith(
        YIELD_PROVIDER_ADDRESS,
        expect.any(Object),
      );
      expect(recordedResult?.isOk()).toBe(true);
    });

    it("submits max available withdrawal requests to beacon chain", async () => {
      // Arrange
      // (processor configured in beforeEach)

      // Act
      await processor.process();

      // Assert
      expect(beaconClient.submitMaxAvailableWithdrawalRequests).toHaveBeenCalledTimes(1);
    });

    it("records operation mode duration in seconds", async () => {
      // Arrange
      const performanceSpy = jest
        .spyOn(performance, "now")
        .mockReturnValueOnce(PERFORMANCE_START)
        .mockReturnValueOnce(PERFORMANCE_END);

      // Act
      await processor.process();

      // Assert
      expect(msToSecondsMock).toHaveBeenCalledWith(PERFORMANCE_END - PERFORMANCE_START);
      expect(metricsUpdater.recordOperationModeDuration).toHaveBeenCalledWith(
        OperationMode.OSSIFICATION_COMPLETE_MODE,
        EXPECTED_DURATION_SECONDS,
      );

      performanceSpy.mockRestore();
    });

    it("records failed withdrawal attempts while continuing processing", async () => {
      // Arrange
      const failure = new Error("withdrawal failed");
      yieldManager.safeMaxAddToWithdrawalReserve.mockRejectedValue(failure);

      // Act
      await processor.process();

      // Assert
      const recordedResult = metricsRecorder.recordSafeWithdrawalMetrics.mock.calls[0]?.[1];
      expect(recordedResult?.isErr()).toBe(true);
    });

    it("continues to beacon chain operations after withdrawal failure", async () => {
      // Arrange
      const failure = new Error("withdrawal failed");
      yieldManager.safeMaxAddToWithdrawalReserve.mockRejectedValue(failure);

      // Act
      await processor.process();

      // Assert
      expect(beaconClient.submitMaxAvailableWithdrawalRequests).toHaveBeenCalledTimes(1);
    });
  });
});
