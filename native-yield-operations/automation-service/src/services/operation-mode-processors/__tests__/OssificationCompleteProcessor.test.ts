import { jest } from "@jest/globals";
import type { ILogger } from "@consensys/linea-shared-utils";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IOperationModeMetricsRecorder } from "../../../core/metrics/IOperationModeMetricsRecorder.js";
import type { IYieldManager } from "../../../core/clients/contracts/IYieldManager.js";
import type { TransactionReceipt, Address } from "viem";
import type { IBeaconChainStakingClient } from "../../../core/clients/IBeaconChainStakingClient.js";
import { OperationMode } from "../../../core/enums/OperationModeEnums.js";
import { OperationTrigger } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { OssificationCompleteProcessor } from "../OssificationCompleteProcessor.js";
import { ResultAsync } from "neverthrow";

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

describe("OssificationCompleteProcessor", () => {
  const yieldProvider = "0x1111111111111111111111111111111111111111" as Address;
  const maxInactionMs = 5_000;

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
    logger = {
      name: "test",
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    } as unknown as jest.Mocked<ILogger>;
    metricsUpdater = {
      incrementOperationModeTrigger: jest.fn(),
      recordOperationModeDuration: jest.fn(),
    } as unknown as jest.Mocked<INativeYieldAutomationMetricsUpdater>;

    metricsRecorder = {
      recordSafeWithdrawalMetrics: jest.fn(),
    } as unknown as jest.Mocked<IOperationModeMetricsRecorder>;

    yieldManager = {
      safeMaxAddToWithdrawalReserve: jest.fn(),
    } as unknown as jest.Mocked<IYieldManager<TransactionReceipt>>;

    beaconClient = {
      submitMaxAvailableWithdrawalRequests: jest.fn(),
    } as unknown as jest.Mocked<IBeaconChainStakingClient>;

    waitMock.mockResolvedValue(undefined);
    msToSecondsMock.mockImplementation((ms: number) => ms / 1_000);
    yieldManager.safeMaxAddToWithdrawalReserve.mockResolvedValue(undefined as unknown as TransactionReceipt);
    attemptMock.mockImplementation(((logger: ILogger, fn: () => unknown | Promise<unknown>) =>
      ResultAsync.fromPromise(
        Promise.resolve().then(() => fn()),
        (error) => error as Error,
      )) as typeof attempt);
  });

  const createProcessor = () =>
    new OssificationCompleteProcessor(
      logger,
      metricsUpdater,
      metricsRecorder,
      yieldManager,
      beaconClient,
      maxInactionMs,
      yieldProvider,
    );

  it("waits, processes withdrawals, submits exits, and records metrics", async () => {
    const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(340);

    const processor = createProcessor();
    await processor.process();

    expect(logger.info).toHaveBeenCalledWith(`Waiting ${maxInactionMs}ms before executing actions`);
    expect(waitMock).toHaveBeenCalledWith(maxInactionMs);
    expect(attemptMock).toHaveBeenCalledWith(
      logger,
      expect.any(Function),
      "_process - safeMaxAddToWithdrawalReserve failed (tolerated)",
    );
    expect(yieldManager.safeMaxAddToWithdrawalReserve).toHaveBeenCalledWith(yieldProvider);
    const recordedResult = metricsRecorder.recordSafeWithdrawalMetrics.mock.calls[0]?.[1];
    expect(recordedResult?.isOk()).toBe(true);
    expect(beaconClient.submitMaxAvailableWithdrawalRequests).toHaveBeenCalledTimes(1);
    expect(metricsUpdater.incrementOperationModeTrigger).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_COMPLETE_MODE,
      OperationTrigger.TIMEOUT,
    );
    expect(msToSecondsMock).toHaveBeenCalledWith(240);
    expect(metricsUpdater.recordOperationModeDuration).toHaveBeenCalledWith(
      OperationMode.OSSIFICATION_COMPLETE_MODE,
      0.24,
    );
    performanceSpy.mockRestore();
  });

  it("records failed withdrawal attempts while continuing processing", async () => {
    const failure = new Error("boom");
    yieldManager.safeMaxAddToWithdrawalReserve.mockRejectedValue(failure);

    const processor = createProcessor();
    await processor.process();

    const recordedResult = metricsRecorder.recordSafeWithdrawalMetrics.mock.calls[0]?.[1];
    expect(recordedResult?.isErr()).toBe(true);
    expect(beaconClient.submitMaxAvailableWithdrawalRequests).toHaveBeenCalledTimes(1);
  });
});
