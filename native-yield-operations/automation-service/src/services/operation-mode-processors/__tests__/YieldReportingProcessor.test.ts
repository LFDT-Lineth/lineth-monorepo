import { jest } from "@jest/globals";
import { ResultAsync } from "neverthrow";
import type { ILogger } from "@consensys/linea-shared-utils";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IOperationModeMetricsRecorder } from "../../../core/metrics/IOperationModeMetricsRecorder.js";
import type { IYieldManager } from "../../../core/clients/contracts/IYieldManager.js";
import type { ILazyOracle } from "../../../core/clients/contracts/ILazyOracle.js";
import type { ILidoAccountingReportClient } from "../../../core/clients/ILidoAccountingReportClient.js";
import type { ILineaRollupYieldExtension } from "../../../core/clients/contracts/ILineaRollupYieldExtension.js";
import type { IBeaconChainStakingClient } from "../../../core/clients/IBeaconChainStakingClient.js";
import type { Address, TransactionReceipt, Hex } from "viem";
import { OperationTrigger } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { OperationMode } from "../../../core/enums/OperationModeEnums.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import { YieldReportingProcessor } from "../YieldReportingProcessor.js";
import type { UpdateVaultDataParams } from "../../../core/clients/contracts/ILazyOracle.js";

jest.mock("@consensys/linea-shared-utils", () => {
  const actual = jest.requireActual("@consensys/linea-shared-utils") as typeof import("@consensys/linea-shared-utils");
  return {
    ...actual,
    attempt: jest.fn(),
    msToSeconds: jest.fn(),
    weiToGweiNumber: jest.fn(),
  };
});

import { attempt, msToSeconds, weiToGweiNumber } from "@consensys/linea-shared-utils";

describe("YieldReportingProcessor", () => {
  const yieldProvider = "0x1111111111111111111111111111111111111111" as Address;
  const l2Recipient = "0x2222222222222222222222222222222222222222" as Address;
  const vaultAddress = "0x3333333333333333333333333333333333333333" as Address;
  const stakeAmount = 10n;
  const submitParams: UpdateVaultDataParams = {
    vault: vaultAddress,
    totalValue: 0n,
    cumulativeLidoFees: 0n,
    liabilityShares: 0n,
    maxLiabilityShares: 0n,
    slashingReserve: 0n,
    proof: [] as Hex[],
  };

  let logger: jest.Mocked<ILogger>;
  let metricsUpdater: jest.Mocked<INativeYieldAutomationMetricsUpdater>;
  let metricsRecorder: jest.Mocked<IOperationModeMetricsRecorder>;
  let yieldManager: jest.Mocked<IYieldManager<TransactionReceipt>>;
  let lazyOracle: jest.Mocked<ILazyOracle<TransactionReceipt>>;
  let lidoReportClient: jest.Mocked<ILidoAccountingReportClient>;
  let yieldExtension: jest.Mocked<ILineaRollupYieldExtension<TransactionReceipt>>;
  let beaconClient: jest.Mocked<IBeaconChainStakingClient>;
  const attemptMock = attempt as jest.MockedFunction<typeof attempt>;
  const msToSecondsMock = msToSeconds as jest.MockedFunction<typeof msToSeconds>;
  const weiToGweiNumberMock = weiToGweiNumber as jest.MockedFunction<typeof weiToGweiNumber>;

  beforeEach(() => {
    jest.clearAllMocks();

    logger = {
      name: "logger",
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    } as unknown as jest.Mocked<ILogger>;

    metricsUpdater = {
      incrementOperationModeTrigger: jest.fn(),
      recordOperationModeDuration: jest.fn(),
      incrementLidoVaultAccountingReport: jest.fn(),
      recordRebalance: jest.fn(),
    } as unknown as jest.Mocked<INativeYieldAutomationMetricsUpdater>;

    metricsRecorder = {
      recordTransferFundsMetrics: jest.fn(),
      recordSafeWithdrawalMetrics: jest.fn(),
      recordReportYieldMetrics: jest.fn(),
    } as unknown as jest.Mocked<IOperationModeMetricsRecorder>;

    yieldManager = {
      getLidoStakingVaultAddress: jest.fn(),
      getRebalanceRequirements: jest.fn(),
      pauseStakingIfNotAlready: jest.fn(),
      unpauseStakingIfNotAlready: jest.fn(),
      fundYieldProvider: jest.fn(),
      safeAddToWithdrawalReserveIfAboveThreshold: jest.fn(),
      reportYield: jest.fn(),
    } as unknown as jest.Mocked<IYieldManager<TransactionReceipt>>;

    lazyOracle = {
      waitForVaultsReportDataUpdatedEvent: jest.fn(),
    } as unknown as jest.Mocked<ILazyOracle<TransactionReceipt>>;

    lidoReportClient = {
      getLatestSubmitVaultReportParams: jest.fn(),
      submitLatestVaultReport: jest.fn(),
    } as unknown as jest.Mocked<ILidoAccountingReportClient>;

    yieldExtension = {
      transferFundsForNativeYield: jest.fn(),
    } as unknown as jest.Mocked<ILineaRollupYieldExtension<TransactionReceipt>>;

    beaconClient = {
      submitWithdrawalRequestsToFulfilAmount: jest.fn(),
    } as unknown as jest.Mocked<IBeaconChainStakingClient>;

    lazyOracle.waitForVaultsReportDataUpdatedEvent.mockResolvedValue({
      result: OperationTrigger.TIMEOUT,
    });
    yieldManager.getLidoStakingVaultAddress.mockResolvedValue(vaultAddress);
    lidoReportClient.getLatestSubmitVaultReportParams.mockResolvedValue(submitParams);
    lidoReportClient.submitLatestVaultReport.mockResolvedValue(undefined);
    yieldManager.reportYield.mockResolvedValue({ transactionHash: "0xyield" } as unknown as TransactionReceipt);
    yieldExtension.transferFundsForNativeYield.mockResolvedValue({
      transactionHash: "0xtransfer",
    } as unknown as TransactionReceipt);
    yieldManager.fundYieldProvider.mockResolvedValue({ transactionHash: "0xfund" } as unknown as TransactionReceipt);
    yieldManager.safeAddToWithdrawalReserveIfAboveThreshold.mockResolvedValue(
      undefined as unknown as TransactionReceipt,
    );

    attemptMock.mockImplementation(((loggerArg: ILogger, fn: () => unknown | Promise<unknown>) =>
      ResultAsync.fromPromise((async () => fn())(), (error) => error as Error)) as typeof attempt);
    msToSecondsMock.mockImplementation((ms: number) => ms / 1_000);
    weiToGweiNumberMock.mockImplementation((value: bigint) => Number(value));
  });

  const createProcessor = (shouldSubmitVaultReport: boolean = true) =>
    new YieldReportingProcessor(
      logger,
      metricsUpdater,
      metricsRecorder,
      yieldManager,
      lazyOracle,
      yieldExtension,
      lidoReportClient,
      beaconClient,
      yieldProvider,
      l2Recipient,
      shouldSubmitVaultReport,
    );

  it("_process - processes staking surplus flow and records metrics", async () => {
    yieldManager.getRebalanceRequirements
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.STAKE, rebalanceAmount: stakeAmount })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.UNSTAKE, rebalanceAmount: 5n });

    const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(250);

    const processor = createProcessor();
    await processor.process();

    expect(lazyOracle.waitForVaultsReportDataUpdatedEvent).toHaveBeenCalledTimes(1);
    expect(metricsUpdater.incrementOperationModeTrigger).toHaveBeenCalledWith(
      OperationMode.YIELD_REPORTING_MODE,
      OperationTrigger.TIMEOUT,
    );
    expect(yieldManager.pauseStakingIfNotAlready).not.toHaveBeenCalled();
    expect(yieldExtension.transferFundsForNativeYield).toHaveBeenCalledWith(stakeAmount);
    expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.STAKE, Number(stakeAmount));
    expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledTimes(1);
    const transferResult = metricsRecorder.recordTransferFundsMetrics.mock.calls[0][1];
    expect(transferResult.isOk()).toBe(true);
    expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(vaultAddress);
    expect(metricsUpdater.incrementLidoVaultAccountingReport).toHaveBeenCalledWith(vaultAddress);
    expect(metricsRecorder.recordReportYieldMetrics).toHaveBeenCalledTimes(1);
    expect(yieldManager.unpauseStakingIfNotAlready).toHaveBeenCalledWith(yieldProvider);
    expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).toHaveBeenCalledWith(5n);
    expect(msToSeconds).toHaveBeenCalledWith(150);
    expect(metricsUpdater.recordOperationModeDuration).toHaveBeenCalledWith(OperationMode.YIELD_REPORTING_MODE, 0.15);

    performanceSpy.mockRestore();
  });

  it("_process - pauses staking when starting in deficit and skips unpause", async () => {
    yieldManager.getRebalanceRequirements
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.UNSTAKE, rebalanceAmount: 8n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n });

    const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(200).mockReturnValueOnce(260);

    const processor = createProcessor();
    await processor.process();

    expect(yieldManager.pauseStakingIfNotAlready).toHaveBeenCalledWith(yieldProvider);
    expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
    expect(yieldManager.unpauseStakingIfNotAlready).not.toHaveBeenCalled();
    expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).not.toHaveBeenCalled();

    performanceSpy.mockRestore();
  });

  it("_process - skips vault report submission when shouldSubmitVaultReport is false", async () => {
    yieldManager.getRebalanceRequirements
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n });

    const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(200);

    const processor = createProcessor(false);
    await processor.process();

    expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
    expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
    expect(logger.info).toHaveBeenCalledWith(
      "_handleSubmitLatestVaultReport: skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)",
    );
    expect(yieldManager.reportYield).toHaveBeenCalledWith(yieldProvider, l2Recipient);

    performanceSpy.mockRestore();
  });

  it("_process - performs an amendment unstake when stake flow flips to deficit mid-cycle", async () => {
    yieldManager.getRebalanceRequirements
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.STAKE, rebalanceAmount: 4n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.UNSTAKE, rebalanceAmount: 6n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n });

    const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(50).mockReturnValueOnce(200);
    const processor = createProcessor();
    const amendmentSpy = jest.spyOn(
      processor as unknown as { _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void> },
      "_handleUnstakingRebalance",
    );

    await processor.process();

    expect(amendmentSpy).toHaveBeenCalledWith(6n, false);
    expect(yieldManager.unpauseStakingIfNotAlready).not.toHaveBeenCalled();

    performanceSpy.mockRestore();
    amendmentSpy.mockRestore();
  });

  it("_process - if start in excess, and no amendment unstake is required, will perform unpause staking", async () => {
    yieldManager.getRebalanceRequirements
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.STAKE, rebalanceAmount: 3n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n });

    const processor = createProcessor();
    const amendmentSpy = jest.spyOn(
      processor as unknown as { _handleUnstakingRebalance(amount: bigint, success: boolean): Promise<void> },
      "_handleUnstakingRebalance",
    );

    await processor.process();

    expect(amendmentSpy).not.toHaveBeenCalled();
    expect(yieldManager.unpauseStakingIfNotAlready).toHaveBeenCalledWith(yieldProvider);

    amendmentSpy.mockRestore();
  });

  it("_process - does not perform ending beacon chain withdrawal if there is no ending deficit", async () => {
    yieldManager.getRebalanceRequirements
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n })
      .mockResolvedValueOnce({ rebalanceDirection: RebalanceDirection.STAKE, rebalanceAmount: 2n });

    const processor = createProcessor();
    await processor.process();

    expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).not.toHaveBeenCalled();
  });

  it("_handleRebalance submits report when no rebalance is needed", async () => {
    const processor = createProcessor();
    const submitSpy = jest
      .spyOn(
        processor as unknown as { _handleSubmitLatestVaultReport(): Promise<unknown> },
        "_handleSubmitLatestVaultReport",
      )
      .mockResolvedValue(undefined);

    await (
      processor as unknown as {
        _handleRebalance(req: unknown): Promise<void>;
      }
    )._handleRebalance({ rebalanceDirection: RebalanceDirection.NONE, rebalanceAmount: 0n });

    expect(submitSpy).toHaveBeenCalledTimes(1);
  });

  it("_handleRebalance routes excess directions to _handleStakingRebalance", async () => {
    const processor = createProcessor();
    const stakingSpy = jest
      .spyOn(
        processor as unknown as { _handleStakingRebalance(amount: bigint): Promise<void> },
        "_handleStakingRebalance",
      )
      .mockResolvedValue(undefined);

    await (
      processor as unknown as {
        _handleRebalance(req: unknown): Promise<void>;
      }
    )._handleRebalance({ rebalanceDirection: RebalanceDirection.STAKE, rebalanceAmount: 42n });

    expect(stakingSpy).toHaveBeenCalledWith(42n);
  });

  it("_handleRebalance routes deficit directions to _handleUnstakingRebalance", async () => {
    const processor = createProcessor();
    const unstakeSpy = jest
      .spyOn(
        processor as unknown as { _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void> },
        "_handleUnstakingRebalance",
      )
      .mockResolvedValue(undefined);

    await (
      processor as unknown as {
        _handleRebalance(req: unknown): Promise<void>;
      }
    )._handleRebalance({ rebalanceDirection: RebalanceDirection.UNSTAKE, rebalanceAmount: 17n });

    expect(unstakeSpy).toHaveBeenCalledWith(17n, true);
  });

  it("_handleStakingRebalance successfully calls rebalance functions and reports yield", async () => {
    const processor = createProcessor();
    const submitSpy = jest
      .spyOn(
        processor as unknown as { _handleSubmitLatestVaultReport(): Promise<unknown> },
        "_handleSubmitLatestVaultReport",
      )
      .mockResolvedValue(undefined);

    await (
      processor as unknown as {
        _handleStakingRebalance(amount: bigint): Promise<void>;
      }
    )._handleStakingRebalance(18n);

    expect(yieldExtension.transferFundsForNativeYield).toHaveBeenCalledWith(18n);
    expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.STAKE, Number(18n));
    expect(yieldManager.fundYieldProvider).toHaveBeenCalledWith(yieldProvider, 18n);
    expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledTimes(1);
    const transferResult = metricsRecorder.recordTransferFundsMetrics.mock.calls[0][1];
    expect(transferResult.isOk()).toBe(true);
    expect(submitSpy).toHaveBeenCalledTimes(1);
    submitSpy.mockRestore();
  });

  it("_handleStakingRebalance tolerates failure of fundYieldProvider", async () => {
    yieldManager.fundYieldProvider.mockRejectedValueOnce(new Error("fund fail"));

    const processor = createProcessor();
    const submitSpy = jest
      .spyOn(
        processor as unknown as { _handleSubmitLatestVaultReport(): Promise<unknown> },
        "_handleSubmitLatestVaultReport",
      )
      .mockResolvedValue(undefined);

    await (
      processor as unknown as {
        _handleStakingRebalance(amount: bigint): Promise<void>;
      }
    )._handleStakingRebalance(11n);

    expect(yieldExtension.transferFundsForNativeYield).toHaveBeenCalledWith(11n);
    expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.STAKE, Number(11n));
    expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledTimes(1);
    const result = metricsRecorder.recordTransferFundsMetrics.mock.calls[0][1];
    expect(result.isErr()).toBe(true);
    expect(submitSpy).toHaveBeenCalledTimes(1);
    submitSpy.mockRestore();
  });

  it("_handleStakingRebalance tolerates failure of transferFundsForNativeYieldResult, but will skip fundYieldProvider and metrics update", async () => {
    yieldExtension.transferFundsForNativeYield.mockRejectedValueOnce(new Error("transfer fail"));

    const processor = createProcessor();
    const submitSpy = jest
      .spyOn(
        processor as unknown as { _handleSubmitLatestVaultReport(): Promise<unknown> },
        "_handleSubmitLatestVaultReport",
      )
      .mockResolvedValue(undefined);

    await (
      processor as unknown as {
        _handleStakingRebalance(amount: bigint): Promise<void>;
      }
    )._handleStakingRebalance(9n);

    expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
    expect(yieldManager.fundYieldProvider).not.toHaveBeenCalled();
    expect(metricsRecorder.recordTransferFundsMetrics).not.toHaveBeenCalled();
    expect(submitSpy).toHaveBeenCalledTimes(1);
    submitSpy.mockRestore();
  });

  it("_handleUnstakingRebalance submits report before withdrawing when shouldReportYield is true", async () => {
    const processor = createProcessor();
    const submitSpy = jest
      .spyOn(
        processor as unknown as { _handleSubmitLatestVaultReport(): Promise<unknown> },
        "_handleSubmitLatestVaultReport",
      )
      .mockResolvedValue(undefined);

    await (
      processor as unknown as {
        _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
      }
    )._handleUnstakingRebalance(15n, true);

    expect(submitSpy).toHaveBeenCalledTimes(1);
    expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
  });

  it("_handleUnstakingRebalance skips report submission when shouldReportYield is false", async () => {
    const processor = createProcessor();
    const submitSpy = jest.spyOn(
      processor as unknown as { _handleSubmitLatestVaultReport(): Promise<unknown> },
      "_handleSubmitLatestVaultReport",
    );

    await (
      processor as unknown as {
        _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
      }
    )._handleUnstakingRebalance(7n, false);

    expect(submitSpy).not.toHaveBeenCalled();
    expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
  });

  it("_handleUnstakingRebalance tolerates failure of safeAddToWithdrawalReserveIfAboveThreshold", async () => {
    const error = new Error("withdrawal failed");
    yieldManager.safeAddToWithdrawalReserveIfAboveThreshold.mockRejectedValueOnce(error);
    const processor = createProcessor();

    await (
      processor as unknown as {
        _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
      }
    )._handleUnstakingRebalance(20n, false);

    expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
    const result = metricsRecorder.recordSafeWithdrawalMetrics.mock.calls[0][1];
    expect(result.isErr()).toBe(true);
  });

  it("_handleSubmitLatestVaultReport continues to report yield when vault submission fails", async () => {
    lidoReportClient.submitLatestVaultReport.mockRejectedValueOnce(new Error("vault fail"));

    const processor = createProcessor();
    (processor as unknown as { vault: Address }).vault = vaultAddress;

    await (
      processor as unknown as {
        _handleSubmitLatestVaultReport(): Promise<void>;
      }
    )._handleSubmitLatestVaultReport();

    expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
    expect(yieldManager.reportYield).toHaveBeenCalledWith(yieldProvider, l2Recipient);
    expect(metricsRecorder.recordReportYieldMetrics).toHaveBeenCalledWith(
      yieldProvider,
      expect.objectContaining({ isOk: expect.any(Function) }),
    );
  });

  it("_handleSubmitLatestVaultReport continues execution when yield reporting fails", async () => {
    yieldManager.reportYield.mockRejectedValueOnce(new Error("yield fail"));

    const processor = createProcessor();
    (processor as unknown as { vault: Address }).vault = vaultAddress;

    await (
      processor as unknown as {
        _handleSubmitLatestVaultReport(): Promise<void>;
      }
    )._handleSubmitLatestVaultReport();

    expect(metricsUpdater.incrementLidoVaultAccountingReport).toHaveBeenCalledWith(vaultAddress);
    expect(logger.info).toHaveBeenCalledWith("_handleSubmitLatestVaultReport: vault report succeeded");
    expect(metricsRecorder.recordReportYieldMetrics).not.toHaveBeenCalled();
  });

  it("_handleSubmitLatestVaultReport logs success when both steps succeed", async () => {
    const processor = createProcessor();
    (processor as unknown as { vault: Address }).vault = vaultAddress;

    await (
      processor as unknown as {
        _handleSubmitLatestVaultReport(): Promise<void>;
      }
    )._handleSubmitLatestVaultReport();

    expect(metricsUpdater.incrementLidoVaultAccountingReport).toHaveBeenCalledWith(vaultAddress);
    expect(logger.info).toHaveBeenCalledWith("_handleSubmitLatestVaultReport: vault report succeeded");
    expect(metricsRecorder.recordReportYieldMetrics).toHaveBeenCalledWith(
      yieldProvider,
      expect.objectContaining({ isOk: expect.any(Function) }),
    );
    expect(logger.info).toHaveBeenCalledWith("_handleSubmitLatestVaultReport: yield report succeeded");
  });

  it("_handleSubmitLatestVaultReport skips vault report submission when shouldSubmitVaultReport is false", async () => {
    const processor = createProcessor(false);
    (processor as unknown as { vault: Address }).vault = vaultAddress;

    await (
      processor as unknown as {
        _handleSubmitLatestVaultReport(): Promise<void>;
      }
    )._handleSubmitLatestVaultReport();

    expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
    expect(logger.info).toHaveBeenCalledWith(
      "_handleSubmitLatestVaultReport: skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)",
    );
    expect(yieldManager.reportYield).toHaveBeenCalledWith(yieldProvider, l2Recipient);
    expect(metricsRecorder.recordReportYieldMetrics).toHaveBeenCalledWith(
      yieldProvider,
      expect.objectContaining({ isOk: expect.any(Function) }),
    );
    expect(logger.info).toHaveBeenCalledWith("_handleSubmitLatestVaultReport: yield report succeeded");
  });
});
