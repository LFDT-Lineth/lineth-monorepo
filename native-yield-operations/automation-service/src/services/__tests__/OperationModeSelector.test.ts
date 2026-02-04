import { jest, describe, it, expect, beforeEach } from "@jest/globals";
import type { ILogger } from "@consensys/linea-shared-utils";
import type { Address, TransactionReceipt } from "viem";

import type { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import type { IOperationModeProcessor } from "../../core/services/operation-mode/IOperationModeProcessor.js";
import { OperationMode } from "../../core/enums/OperationModeEnums.js";
import { OperationModeExecutionStatus } from "../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";

jest.mock("@consensys/linea-shared-utils", () => {
  const actual = jest.requireActual<typeof import("@consensys/linea-shared-utils")>("@consensys/linea-shared-utils");
  return {
    ...actual,
    wait: jest.fn(),
  };
});

import { wait } from "@consensys/linea-shared-utils";
import { OperationModeSelector } from "../OperationModeSelector.js";

const createLoggerMock = (): jest.Mocked<ILogger> => ({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

describe("OperationModeSelector", () => {
  // Test constants
  const TEST_YIELD_PROVIDER = "0x1111111111111111111111111111111111111111" as Address;
  const TEST_RETRY_TIME_MS = 123;
  const CUSTOM_RETRY_TIME_MS = 321;

  let selector: OperationModeSelector;
  let logger: jest.Mocked<ILogger>;
  let metricsUpdater: jest.Mocked<INativeYieldAutomationMetricsUpdater>;
  let yieldManager: jest.Mocked<IYieldManager<TransactionReceipt>>;
  let yieldReportingProcessor: jest.Mocked<IOperationModeProcessor>;
  let ossificationPendingProcessor: jest.Mocked<IOperationModeProcessor>;
  let ossificationCompleteProcessor: jest.Mocked<IOperationModeProcessor>;
  let waitMock: jest.MockedFunction<typeof wait>;

  const createSelector = (retryTime: number = TEST_RETRY_TIME_MS): OperationModeSelector =>
    new OperationModeSelector(
      logger,
      metricsUpdater,
      yieldManager,
      yieldReportingProcessor,
      ossificationPendingProcessor,
      ossificationCompleteProcessor,
      TEST_YIELD_PROVIDER,
      retryTime,
    );

  beforeEach(() => {
    jest.clearAllMocks();

    logger = createLoggerMock();
    metricsUpdater = {
      recordRebalance: jest.fn(),
      addValidatorPartialUnstakeAmount: jest.fn(),
      incrementValidatorExit: jest.fn(),
      setValidatorStakedAmountGwei: jest.fn(),
      incrementLidoVaultAccountingReport: jest.fn(),
      incrementReportYield: jest.fn(),
      setLastPeekedNegativeYieldReport: jest.fn(),
      setLastPeekedPositiveYieldReport: jest.fn(),
      setLastSettleableLidoFees: jest.fn(),
      setLastVaultReportTimestamp: jest.fn(),
      setYieldReportedCumulative: jest.fn(),
      setLstLiabilityPrincipalGwei: jest.fn(),
      setLastReportedNegativeYield: jest.fn(),
      setLidoLstLiabilityGwei: jest.fn(),
      setLastTotalPendingPartialWithdrawalsGwei: jest.fn(),
      setLastTotalValidatorBalanceGwei: jest.fn(),
      setLastTotalPendingDepositGwei: jest.fn(),
      setPendingPartialWithdrawalQueueAmountGwei: jest.fn(),
      setPendingDepositQueueAmountGwei: jest.fn(),
      setPendingExitQueueAmountGwei: jest.fn(),
      setLastTotalPendingExitGwei: jest.fn(),
      setPendingFullWithdrawalQueueAmountGwei: jest.fn(),
      setLastTotalPendingFullWithdrawalGwei: jest.fn(),
      addNodeOperatorFeesPaid: jest.fn(),
      addLiabilitiesPaid: jest.fn(),
      addLidoFeesPaid: jest.fn(),
      incrementOperationModeExecution: jest.fn(),
      recordOperationModeDuration: jest.fn(),
      incrementStakingDepositQuotaExceeded: jest.fn(),
      setActualRebalanceRequirement: jest.fn(),
      setReportedRebalanceRequirement: jest.fn(),
      incrementContractEstimateGasError: jest.fn(),
    } as jest.Mocked<INativeYieldAutomationMetricsUpdater>;
    yieldManager = {
      isOssificationInitiated: jest.fn(),
      isOssified: jest.fn(),
    } as unknown as jest.Mocked<IYieldManager<TransactionReceipt>>;
    yieldReportingProcessor = {
      process: jest.fn(),
    } as jest.Mocked<IOperationModeProcessor>;
    ossificationPendingProcessor = {
      process: jest.fn(),
    } as jest.Mocked<IOperationModeProcessor>;
    ossificationCompleteProcessor = {
      process: jest.fn(),
    } as jest.Mocked<IOperationModeProcessor>;

    waitMock = wait as jest.MockedFunction<typeof wait>;
    waitMock.mockResolvedValue(undefined);
  });

  it("executes yield reporting mode when neither ossification flag is set", async () => {
    // Arrange
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(false);
    yieldManager.isOssified.mockResolvedValueOnce(false);

    selector = createSelector();
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    // Act
    await selector.start();

    // Assert
    expect(yieldReportingProcessor.process).toHaveBeenCalledTimes(1);
    expect(ossificationPendingProcessor.process).not.toHaveBeenCalled();
    expect(ossificationCompleteProcessor.process).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.YIELD_REPORTING_MODE,
      OperationModeExecutionStatus.Success,
    );
    expect(waitMock).not.toHaveBeenCalled();
  });

  it("executes ossification pending mode when ossification is initiated", async () => {
    // Arrange
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(true);
    yieldManager.isOssified.mockResolvedValueOnce(false);

    selector = createSelector();
    ossificationPendingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    // Act
    await selector.start();

    // Assert
    expect(ossificationPendingProcessor.process).toHaveBeenCalledTimes(1);
    expect(yieldReportingProcessor.process).not.toHaveBeenCalled();
    expect(ossificationCompleteProcessor.process).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_PENDING_MODE,
      OperationModeExecutionStatus.Success,
    );
  });

  it("executes ossification complete mode when ossified", async () => {
    // Arrange
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(true);
    yieldManager.isOssified.mockResolvedValueOnce(true);

    selector = createSelector();
    ossificationCompleteProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    // Act
    await selector.start();

    // Assert
    expect(ossificationCompleteProcessor.process).toHaveBeenCalledTimes(1);
    expect(yieldReportingProcessor.process).not.toHaveBeenCalled();
    expect(ossificationPendingProcessor.process).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_COMPLETE_MODE,
      OperationModeExecutionStatus.Success,
    );
  });

  it("skips start when already running", async () => {
    // Arrange
    yieldManager.isOssificationInitiated.mockResolvedValue(false);
    yieldManager.isOssified.mockResolvedValue(false);

    selector = createSelector();
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    // Act
    await Promise.all([selector.start(), selector.start()]);

    // Assert
    expect(logger.debug).toHaveBeenCalledWith("OperationModeSelector.start() - already running, skipping");
    expect(yieldReportingProcessor.process).toHaveBeenCalledTimes(1);
  });

  it("skips stop when not running", () => {
    // Arrange
    selector = createSelector();

    // Act
    selector.stop();

    // Assert
    expect(logger.debug).toHaveBeenCalledWith("OperationModeSelector.stop() - not running, skipping");
    expect(logger.info).not.toHaveBeenCalled();
  });

  it("retries after contract read error", async () => {
    // Arrange
    const contractError = new Error("contract read failed");

    yieldManager.isOssificationInitiated.mockRejectedValueOnce(contractError);
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(false);
    yieldManager.isOssified.mockRejectedValueOnce(contractError);
    yieldManager.isOssified.mockResolvedValueOnce(false);

    selector = createSelector(CUSTOM_RETRY_TIME_MS);
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    // Act
    await selector.start();

    // Assert
    expect(logger.error).toHaveBeenCalledWith(
      `selectOperationModeLoop error, retrying in ${CUSTOM_RETRY_TIME_MS}ms`,
      { error: contractError },
    );
    expect(waitMock).toHaveBeenCalledWith(CUSTOM_RETRY_TIME_MS);
    expect(yieldReportingProcessor.process).toHaveBeenCalledTimes(1);
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.YIELD_REPORTING_MODE,
      OperationModeExecutionStatus.Success,
    );
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.UNKNOWN,
      OperationModeExecutionStatus.Failure,
    );
  });

  it("records UNKNOWN failure metric when contract read fails", async () => {
    // Arrange
    const contractError = new Error("contract read failed");

    yieldManager.isOssificationInitiated.mockRejectedValueOnce(contractError);
    yieldManager.isOssificationInitiated.mockResolvedValueOnce(false);
    yieldManager.isOssified.mockResolvedValueOnce(false);

    selector = createSelector();
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    // Act
    await selector.start();

    // Assert
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.UNKNOWN,
      OperationModeExecutionStatus.Failure,
    );
  });

  it("records YIELD_REPORTING_MODE failure metric when processor fails", async () => {
    // Arrange
    const processorError = new Error("processor failed");

    yieldManager.isOssificationInitiated.mockResolvedValue(false);
    yieldManager.isOssified.mockResolvedValue(false);
    yieldReportingProcessor.process.mockRejectedValueOnce(processorError);
    yieldReportingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    selector = createSelector();

    // Act
    await selector.start();

    // Assert
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.YIELD_REPORTING_MODE,
      OperationModeExecutionStatus.Failure,
    );
  });

  it("records OSSIFICATION_PENDING_MODE failure metric when pending processor fails", async () => {
    // Arrange
    const processorError = new Error("pending processor failed");

    yieldManager.isOssificationInitiated.mockResolvedValue(true);
    yieldManager.isOssified.mockResolvedValue(false);
    ossificationPendingProcessor.process.mockRejectedValueOnce(processorError);
    ossificationPendingProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    selector = createSelector();

    // Act
    await selector.start();

    // Assert
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_PENDING_MODE,
      OperationModeExecutionStatus.Failure,
    );
  });

  it("records OSSIFICATION_COMPLETE_MODE failure metric when complete processor fails", async () => {
    // Arrange
    const processorError = new Error("complete processor failed");

    yieldManager.isOssificationInitiated.mockResolvedValue(true);
    yieldManager.isOssified.mockResolvedValue(true);
    ossificationCompleteProcessor.process.mockRejectedValueOnce(processorError);
    ossificationCompleteProcessor.process.mockImplementation(async () => {
      selector.stop();
    });

    selector = createSelector();

    // Act
    await selector.start();

    // Assert
    expect(metricsUpdater.incrementOperationModeExecution).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_COMPLETE_MODE,
      OperationModeExecutionStatus.Failure,
    );
  });
});
