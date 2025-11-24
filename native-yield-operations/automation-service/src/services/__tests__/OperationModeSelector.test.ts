import { mock, MockProxy } from "jest-mock-extended";
import type { ILogger } from "@consensys/linea-shared-utils";
import type { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import type { TransactionReceipt, Address } from "viem";
import type { IOperationModeProcessor } from "../../core/services/operation-mode/IOperationModeProcessor.js";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";

jest.mock("@consensys/linea-shared-utils", () => {
  const actual = jest.requireActual("@consensys/linea-shared-utils");
  return {
    ...actual,
    wait: jest.fn(),
  };
});

import { wait } from "@consensys/linea-shared-utils";
import { OperationModeSelector } from "../OperationModeSelector.js";

describe("OperationModeSelector", () => {
  const yieldProvider = "0x1111111111111111111111111111111111111111" as Address;

  let logger: MockProxy<ILogger>;
  let metricsUpdater: MockProxy<INativeYieldAutomationMetricsUpdater>;
  let yieldManager: MockProxy<IYieldManager<TransactionReceipt>>;
  let yieldReportingProcessor: MockProxy<IOperationModeProcessor>;
  let ossificationPendingProcessor: MockProxy<IOperationModeProcessor>;
  let ossificationCompleteProcessor: MockProxy<IOperationModeProcessor>;
  let waitMock: jest.MockedFunction<typeof wait>;

  const contractReadRetryTimeMs = 123;

  const createSelector = (retryTime = contractReadRetryTimeMs) =>
    new OperationModeSelector(
      logger,
      metricsUpdater,
      yieldManager,
      yieldReportingProcessor,
      ossificationPendingProcessor,
      ossificationCompleteProcessor,
      yieldProvider,
      retryTime,
    );

  beforeEach(() => {
    jest.clearAllMocks();
    logger = mock<ILogger>();
    metricsUpdater = mock<INativeYieldAutomationMetricsUpdater>();
    yieldManager = mock<IYieldManager<TransactionReceipt>>();
    yieldReportingProcessor = mock<IOperationModeProcessor>();
    ossificationPendingProcessor = mock<IOperationModeProcessor>();
    ossificationCompleteProcessor = mock<IOperationModeProcessor>();

    waitMock = wait as jest.MockedFunction<typeof wait>;
    waitMock.mockResolvedValue(undefined);
  });

  it("runs yield reporting mode when neither ossification flag is set", async () => {
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(false);
    yieldManager.isOssified.mockResolvedValueOnce(false);

    const selector = createSelector();
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    await selector.start();

    expect(yieldReportingProcessor.process).toHaveBeenCalledTimes(1);
    expect(ossificationPendingProcessor.process).not.toHaveBeenCalled();
    expect(ossificationCompleteProcessor.process).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(OperationMode.YIELD_REPORTING_MODE);
    expect(waitMock).not.toHaveBeenCalled();
  });

  it("runs ossification pending mode when ossification is initiated", async () => {
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(true);
    yieldManager.isOssified.mockResolvedValueOnce(false);

    const selector = createSelector();
    ossificationPendingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    await selector.start();

    expect(ossificationPendingProcessor.process).toHaveBeenCalledTimes(1);
    expect(yieldReportingProcessor.process).not.toHaveBeenCalled();
    expect(ossificationCompleteProcessor.process).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_PENDING_MODE,
    );
  });

  it("prefers ossification complete mode when ossified", async () => {
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(true);
    yieldManager.isOssified.mockResolvedValueOnce(true);

    const selector = createSelector();
    ossificationCompleteProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    await selector.start();

    expect(ossificationCompleteProcessor.process).toHaveBeenCalledTimes(1);
    expect(yieldReportingProcessor.process).not.toHaveBeenCalled();
    expect(ossificationPendingProcessor.process).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_COMPLETE_MODE,
    );
  });

  it("is a no-op when start is invoked while already running", async () => {
    yieldManager.isOssificationInitiated.mockResolvedValue(false);
    yieldManager.isOssified.mockResolvedValue(false);

    const selector = createSelector();
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    await Promise.all([selector.start(), selector.start()]);

    expect(yieldReportingProcessor.process).toHaveBeenCalledTimes(1);
  });

  it("does nothing when stop is called before start", () => {
    const selector = createSelector();

    selector.stop();

    expect(logger.info).not.toHaveBeenCalled();
  });

  it("logs errors, waits, and retries before succeeding", async () => {
    const error = new Error("boom");
    const retryTime = 321;

    yieldManager.isOssificationInitiated.mockRejectedValueOnce(error);
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(false);
    yieldManager.isOssified.mockRejectedValueOnce(error);
    yieldManager.isOssified.mockResolvedValueOnce(false);

    const selector = createSelector(retryTime);
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    await selector.start();

    expect(logger.error).toHaveBeenCalledWith(`selectOperationModeLoop error, retrying in ${retryTime}ms`, { error });
    expect(waitMock).toHaveBeenCalledWith(retryTime);
    expect(yieldReportingProcessor.process).toHaveBeenCalledTimes(1);
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(OperationMode.YIELD_REPORTING_MODE);
  });
});
