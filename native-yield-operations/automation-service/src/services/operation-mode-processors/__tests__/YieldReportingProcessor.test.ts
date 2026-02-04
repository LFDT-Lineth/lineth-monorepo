import { ILogger } from "@consensys/linea-shared-utils";
import { jest, describe, it, expect, beforeEach } from "@jest/globals";
import { ResultAsync } from "neverthrow";
import type { Address, TransactionReceipt, Hex } from "viem";

import { createLoggerMock, createMetricsUpdaterMock } from "../../../__tests__/helpers/index.js";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IOperationModeMetricsRecorder } from "../../../core/metrics/IOperationModeMetricsRecorder.js";
import type { IYieldManager } from "../../../core/clients/contracts/IYieldManager.js";
import type { ILazyOracle } from "../../../core/clients/contracts/ILazyOracle.js";
import type { ILidoAccountingReportClient } from "../../../core/clients/ILidoAccountingReportClient.js";
import type { ILineaRollupYieldExtension } from "../../../core/clients/contracts/ILineaRollupYieldExtension.js";
import type { IBeaconChainStakingClient } from "../../../core/clients/IBeaconChainStakingClient.js";
import type { IVaultHub } from "../../../core/clients/contracts/IVaultHub.js";
import type { UpdateVaultDataParams } from "../../../core/clients/contracts/ILazyOracle.js";
import type { YieldReport } from "../../../core/entities/YieldReport.js";
import { OperationTrigger } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { OperationMode } from "../../../core/enums/OperationModeEnums.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import { YieldReportingProcessor } from "../YieldReportingProcessor.js";

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

// Semantic constants
const YIELD_PROVIDER = "0x1111111111111111111111111111111111111111" as Address;
const L2_RECIPIENT = "0x2222222222222222222222222222222222222222" as Address;
const VAULT_ADDRESS = "0x3333333333333333333333333333333333333333" as Address;
const STAKE_AMOUNT = 10n;
const MIN_NEGATIVE_YIELD_THRESHOLD = 1000000000000000000n; // 1 ETH
const ONE_ETH = 1000000000000000000n;
const TWO_ETH = 2000000000000000000n;
const HALF_ETH = 500000000000000000n;
const SETTLEABLE_FEES_ABOVE_THRESHOLD = 600000000000000000n;
const SETTLEABLE_FEES_BELOW_THRESHOLD = 300000000000000000n;
const DEFAULT_CYCLES_PER_YIELD_REPORT = 12;

// Factory functions
const createSubmitParams = (vault: Address): UpdateVaultDataParams => ({
  vault,
  totalValue: 0n,
  cumulativeLidoFees: 0n,
  liabilityShares: 0n,
  maxLiabilityShares: 0n,
  slashingReserve: 0n,
  proof: [] as Hex[],
});

const createYieldReport = (
  yieldAmount: bigint,
  outstandingNegativeYield: bigint,
  yieldProvider: Address,
): YieldReport => ({
  yieldAmount,
  outstandingNegativeYield,
  yieldProvider,
});

const createRebalanceRequirement = (direction: RebalanceDirection, amount: bigint) => ({
  rebalanceDirection: direction,
  rebalanceAmount: amount,
});

const createTransactionReceipt = (hash: string): TransactionReceipt =>
  ({ transactionHash: hash }) as unknown as TransactionReceipt;

const createYieldProviderData = (lastReportedNegativeYield: bigint = 0n) => ({
  yieldProviderVendor: 0,
  isStakingPaused: false,
  isOssificationInitiated: false,
  isOssified: false,
  primaryEntrypoint: "0x0000000000000000000000000000000000000000" as Address,
  ossifiedEntrypoint: "0x0000000000000000000000000000000000000000" as Address,
  yieldProviderIndex: 0n,
  userFunds: 0n,
  yieldReportedCumulative: 0n,
  lstLiabilityPrincipal: 0n,
  lastReportedNegativeYield,
});

const createMetricsRecorderMock = () =>
  ({
    recordTransferFundsMetrics: jest.fn(),
    recordSafeWithdrawalMetrics: jest.fn(),
    recordReportYieldMetrics: jest.fn(),
    recordProgressOssificationMetrics: jest.fn(),
  }) as jest.Mocked<IOperationModeMetricsRecorder>;

describe("YieldReportingProcessor", () => {
  let logger: jest.Mocked<ILogger>;
  let metricsUpdater: jest.Mocked<INativeYieldAutomationMetricsUpdater>;
  let metricsRecorder: jest.Mocked<IOperationModeMetricsRecorder>;
  let yieldManager: jest.Mocked<IYieldManager<TransactionReceipt>>;
  let lazyOracle: jest.Mocked<ILazyOracle<TransactionReceipt>>;
  let lidoReportClient: jest.Mocked<ILidoAccountingReportClient>;
  let yieldExtension: jest.Mocked<ILineaRollupYieldExtension<TransactionReceipt>>;
  let beaconClient: jest.Mocked<IBeaconChainStakingClient>;
  let vaultHubClient: jest.Mocked<IVaultHub<TransactionReceipt>>;
  const attemptMock = attempt as jest.MockedFunction<typeof attempt>;
  const msToSecondsMock = msToSeconds as jest.MockedFunction<typeof msToSeconds>;
  const weiToGweiNumberMock = weiToGweiNumber as jest.MockedFunction<typeof weiToGweiNumber>;

  beforeEach(() => {
    jest.clearAllMocks();

    logger = createLoggerMock() as jest.Mocked<ILogger>;
    metricsUpdater = createMetricsUpdaterMock() as jest.Mocked<INativeYieldAutomationMetricsUpdater>;
    metricsRecorder = createMetricsRecorderMock();

    yieldManager = {
      getLidoStakingVaultAddress: jest.fn(),
      getRebalanceRequirements: jest.fn(),
      pauseStakingIfNotAlready: jest.fn(),
      unpauseStakingIfNotAlready: jest.fn(),
      fundYieldProvider: jest.fn(),
      safeAddToWithdrawalReserveIfAboveThreshold: jest.fn(),
      reportYield: jest.fn(),
      peekYieldReport: jest.fn(),
      getYieldProviderData: jest.fn(),
      getBalance: jest.fn(),
      getTargetReserveDeficit: jest.fn(),
      safeWithdrawFromYieldProvider: jest.fn(),
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

    vaultHubClient = {
      settleableLidoFeesValue: jest.fn(),
      isReportFresh: jest.fn(),
      isVaultConnected: jest.fn(),
    } as unknown as jest.Mocked<IVaultHub<TransactionReceipt>>;

    lazyOracle.waitForVaultsReportDataUpdatedEvent.mockResolvedValue({
      result: OperationTrigger.TIMEOUT,
    });
    yieldManager.getLidoStakingVaultAddress.mockResolvedValue(VAULT_ADDRESS);
    yieldManager.getBalance.mockResolvedValue(0n);
    yieldManager.getTargetReserveDeficit.mockResolvedValue(0n);
    yieldManager.safeWithdrawFromYieldProvider.mockResolvedValue(createTransactionReceipt("0xwithdraw"));
    lidoReportClient.getLatestSubmitVaultReportParams.mockResolvedValue(createSubmitParams(VAULT_ADDRESS));
    lidoReportClient.submitLatestVaultReport.mockResolvedValue(undefined);
    yieldManager.reportYield.mockResolvedValue(createTransactionReceipt("0xyield"));
    yieldExtension.transferFundsForNativeYield.mockResolvedValue(createTransactionReceipt("0xtransfer"));
    yieldManager.fundYieldProvider.mockResolvedValue(createTransactionReceipt("0xfund"));
    yieldManager.safeAddToWithdrawalReserveIfAboveThreshold.mockResolvedValue(
      undefined as unknown as TransactionReceipt,
    );
    yieldManager.getYieldProviderData.mockResolvedValue(createYieldProviderData());

    vaultHubClient.isVaultConnected.mockResolvedValue(true);
    vaultHubClient.isReportFresh.mockResolvedValue(false);

    attemptMock.mockImplementation(((loggerArg: ILogger, fn: () => unknown | Promise<unknown>) =>
      ResultAsync.fromPromise((async () => fn())(), (error) => error as Error)) as typeof attempt);
    msToSecondsMock.mockImplementation((ms: number) => ms / 1_000);
    weiToGweiNumberMock.mockImplementation((value: bigint) => Number(value));
  });

  const createProcessor = (
    shouldSubmitVaultReport: boolean = true,
    shouldReportYield: boolean = true,
    isUnpauseStakingEnabled: boolean = true,
    minNegativeYieldDiffToReportYieldWei: bigint = MIN_NEGATIVE_YIELD_THRESHOLD,
    minWithdrawalThresholdEth: bigint = 0n,
    cyclesPerYieldReport: number = DEFAULT_CYCLES_PER_YIELD_REPORT,
  ) =>
    new YieldReportingProcessor(
      logger,
      metricsUpdater,
      metricsRecorder,
      yieldManager,
      lazyOracle,
      yieldExtension,
      lidoReportClient,
      beaconClient,
      vaultHubClient,
      YIELD_PROVIDER,
      L2_RECIPIENT,
      shouldSubmitVaultReport,
      shouldReportYield,
      isUnpauseStakingEnabled,
      minNegativeYieldDiffToReportYieldWei,
      minWithdrawalThresholdEth,
      cyclesPerYieldReport,
    );

  describe("process", () => {
    it("processes staking surplus flow and records metrics", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.STAKE, STAKE_AMOUNT))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.UNSTAKE, 5n));
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_ABOVE_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(createYieldReport(TWO_ETH, 0n, YIELD_PROVIDER));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(250);
      const processor = createProcessor();
      (processor as any).cycleCount = 11;

      // Act
      await processor.process();

      // Assert
      expect(lazyOracle.waitForVaultsReportDataUpdatedEvent).toHaveBeenCalledTimes(1);
      expect(yieldManager.pauseStakingIfNotAlready).not.toHaveBeenCalled();
      expect(yieldExtension.transferFundsForNativeYield).toHaveBeenCalledWith(STAKE_AMOUNT);
      expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.STAKE, Number(STAKE_AMOUNT));
      expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledTimes(1);
      const transferResult = metricsRecorder.recordTransferFundsMetrics.mock.calls[0][1];
      expect(transferResult.isOk()).toBe(true);
      expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsUpdater.incrementLidoVaultAccountingReport).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsRecorder.recordReportYieldMetrics).toHaveBeenCalledTimes(1);
      expect(yieldManager.unpauseStakingIfNotAlready).toHaveBeenCalledWith(YIELD_PROVIDER);
      expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).toHaveBeenCalledWith(5n);
      expect(msToSeconds).toHaveBeenCalledWith(150);
      expect(metricsUpdater.recordOperationModeDuration).toHaveBeenCalledWith(OperationMode.YIELD_REPORTING_MODE, 0.15);

      performanceSpy.mockRestore();
    });

    it("pauses staking when starting in deficit and skips unpause", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.UNSTAKE, 8n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(200).mockReturnValueOnce(260);
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(yieldManager.pauseStakingIfNotAlready).toHaveBeenCalledWith(YIELD_PROVIDER);
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
      expect(yieldManager.unpauseStakingIfNotAlready).not.toHaveBeenCalled();
      expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).not.toHaveBeenCalled();

      performanceSpy.mockRestore();
    });

    it("skips vault report submission when shouldSubmitVaultReport is false", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_ABOVE_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(createYieldReport(TWO_ETH, 0n, YIELD_PROVIDER));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(200);
      const processor = createProcessor(false, true, true);
      (processor as any).cycleCount = 11;

      // Act
      await processor.process();

      // Assert
      expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
      expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith(
        "_handleSubmitLatestVaultReport - Skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)",
      );
      expect(yieldManager.reportYield).toHaveBeenCalledWith(YIELD_PROVIDER, L2_RECIPIENT);
      expect(metricsRecorder.recordReportYieldMetrics).toHaveBeenCalledTimes(1);

      performanceSpy.mockRestore();
    });

    it("performs amendment unstake when flow flips from STAKE to UNSTAKE mid-cycle", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.STAKE, 4n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.UNSTAKE, 6n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(50).mockReturnValueOnce(200);
      const processor = createProcessor();
      const amendmentSpy = jest.spyOn(
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        },
        "_handleUnstakingRebalance",
      );

      // Act
      await processor.process();

      // Assert
      expect(amendmentSpy).toHaveBeenCalledWith(6n, false);
      expect(yieldManager.unpauseStakingIfNotAlready).not.toHaveBeenCalled();

      performanceSpy.mockRestore();
      amendmentSpy.mockRestore();
    });

    it("performs amendment unstake when flow flips from NONE to UNSTAKE mid-cycle", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.UNSTAKE, 5n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(250);
      const processor = createProcessor();
      const amendmentSpy = jest.spyOn(
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        },
        "_handleUnstakingRebalance",
      );

      // Act
      await processor.process();

      // Assert
      expect(amendmentSpy).toHaveBeenCalledWith(5n, false);
      expect(yieldManager.unpauseStakingIfNotAlready).not.toHaveBeenCalled();

      performanceSpy.mockRestore();
      amendmentSpy.mockRestore();
    });

    it("unpauses staking when starting in excess and no amendment unstake is required", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.STAKE, 3n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      const processor = createProcessor();
      const amendmentSpy = jest.spyOn(
        processor as unknown as { _handleUnstakingRebalance(amount: bigint, success: boolean): Promise<void> },
        "_handleUnstakingRebalance",
      );

      // Act
      await processor.process();

      // Assert
      expect(amendmentSpy).not.toHaveBeenCalled();
      expect(yieldManager.unpauseStakingIfNotAlready).toHaveBeenCalledWith(YIELD_PROVIDER);

      amendmentSpy.mockRestore();
    });

    it("does not perform ending beacon chain withdrawal when there is no ending deficit", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.STAKE, 2n));
      const processor = createProcessor();

      // Act
      await processor.process();

      // Assert
      expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).not.toHaveBeenCalled();
      expect(yieldManager.unpauseStakingIfNotAlready).toHaveBeenCalledWith(YIELD_PROVIDER);
    });

    it("unpauses staking when starting and ending in NONE state", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_ABOVE_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(createYieldReport(TWO_ETH, 0n, YIELD_PROVIDER));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(200);
      const processor = createProcessor();
      const amendmentSpy = jest.spyOn(
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        },
        "_handleUnstakingRebalance",
      );

      // Act
      await processor.process();

      // Assert
      expect(amendmentSpy).not.toHaveBeenCalled();
      expect(yieldManager.unpauseStakingIfNotAlready).toHaveBeenCalledWith(YIELD_PROVIDER);
      expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).not.toHaveBeenCalled();

      performanceSpy.mockRestore();
      amendmentSpy.mockRestore();
    });

    it("does not unpause staking when isUnpauseStakingEnabled is false in STAKE to NONE flow", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.STAKE, 3n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(200);
      const processor = createProcessor(true, true, false);
      const amendmentSpy = jest.spyOn(
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        },
        "_handleUnstakingRebalance",
      );

      // Act
      await processor.process();

      // Assert
      expect(amendmentSpy).not.toHaveBeenCalled();
      expect(yieldManager.unpauseStakingIfNotAlready).not.toHaveBeenCalled();

      performanceSpy.mockRestore();
      amendmentSpy.mockRestore();
    });

    it("does not unpause staking when isUnpauseStakingEnabled is false in NONE to NONE flow", async () => {
      // Arrange
      yieldManager.getRebalanceRequirements
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n))
        .mockResolvedValueOnce(createRebalanceRequirement(RebalanceDirection.NONE, 0n));
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_ABOVE_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(createYieldReport(TWO_ETH, 0n, YIELD_PROVIDER));
      const performanceSpy = jest.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(200);
      const processor = createProcessor(true, true, false);
      const amendmentSpy = jest.spyOn(
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        },
        "_handleUnstakingRebalance",
      );

      // Act
      await processor.process();

      // Assert
      expect(amendmentSpy).not.toHaveBeenCalled();
      expect(yieldManager.unpauseStakingIfNotAlready).not.toHaveBeenCalled();
      expect(beaconClient.submitWithdrawalRequestsToFulfilAmount).not.toHaveBeenCalled();

      performanceSpy.mockRestore();
      amendmentSpy.mockRestore();
    });
  });

  describe("_handleRebalance", () => {
    it("reports yield when no rebalance is needed", async () => {
      // Arrange
      const processor = createProcessor();
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleRebalance(req: unknown): Promise<void>;
        }
      )._handleRebalance(createRebalanceRequirement(RebalanceDirection.NONE, 0n));

      // Assert
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);
    });

    it("routes STAKE direction to staking rebalance handler", async () => {
      // Arrange
      const processor = createProcessor();
      const stakingSpy = jest
        .spyOn(
          processor as unknown as { _handleStakingRebalance(amount: bigint): Promise<void> },
          "_handleStakingRebalance",
        )
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleRebalance(req: unknown): Promise<void>;
        }
      )._handleRebalance(createRebalanceRequirement(RebalanceDirection.STAKE, 42n));

      // Assert
      expect(stakingSpy).toHaveBeenCalledWith(42n);
    });

    it("routes UNSTAKE direction to unstaking rebalance handler", async () => {
      // Arrange
      const processor = createProcessor();
      const unstakeSpy = jest
        .spyOn(
          processor as unknown as {
            _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
          },
          "_handleUnstakingRebalance",
        )
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleRebalance(req: unknown): Promise<void>;
        }
      )._handleRebalance(createRebalanceRequirement(RebalanceDirection.UNSTAKE, 17n));

      // Assert
      expect(unstakeSpy).toHaveBeenCalledWith(17n, true);
    });
  });

  describe("_handleNoRebalance", () => {
    it("transfers YieldManager balance when above threshold", async () => {
      // Arrange
      const minWithdrawalThresholdEth = 1n;
      const yieldManagerBalance = TWO_ETH;
      const processor = createProcessor(true, true, true, MIN_NEGATIVE_YIELD_THRESHOLD, minWithdrawalThresholdEth);
      yieldManager.getBalance.mockResolvedValueOnce(yieldManagerBalance);
      yieldManager.getTargetReserveDeficit.mockResolvedValueOnce(0n);
      yieldManager.fundYieldProvider.mockResolvedValueOnce(createTransactionReceipt("0xtransfer"));
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleRebalance(req: unknown): Promise<void>;
        }
      )._handleRebalance(createRebalanceRequirement(RebalanceDirection.NONE, 0n));

      // Assert
      expect(yieldManager.getBalance).toHaveBeenCalledTimes(1);
      expect(yieldManager.fundYieldProvider).toHaveBeenCalledWith(YIELD_PROVIDER, yieldManagerBalance);
      expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledWith(YIELD_PROVIDER, expect.any(Object));
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);
    });

    it("skips transfer when YieldManager balance is below threshold", async () => {
      // Arrange
      const minWithdrawalThresholdEth = 1n;
      const yieldManagerBalance = HALF_ETH;
      const processor = createProcessor(true, true, true, MIN_NEGATIVE_YIELD_THRESHOLD, minWithdrawalThresholdEth);
      yieldManager.getBalance.mockResolvedValueOnce(yieldManagerBalance);
      yieldManager.getTargetReserveDeficit.mockResolvedValueOnce(0n);
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleRebalance(req: unknown): Promise<void>;
        }
      )._handleRebalance(createRebalanceRequirement(RebalanceDirection.NONE, 0n));

      // Assert
      expect(yieldManager.getBalance).toHaveBeenCalledTimes(1);
      expect(yieldManager.fundYieldProvider).not.toHaveBeenCalled();
      expect(metricsRecorder.recordTransferFundsMetrics).not.toHaveBeenCalled();
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);
    });

    it("withdraws from yield provider when targetReserveDeficit is greater than zero", async () => {
      // Arrange
      const minWithdrawalThresholdEth = 1n;
      const yieldManagerBalance = TWO_ETH;
      const targetReserveDeficit = HALF_ETH;
      const processor = createProcessor(true, true, true, MIN_NEGATIVE_YIELD_THRESHOLD, minWithdrawalThresholdEth);
      yieldManager.getBalance.mockResolvedValueOnce(yieldManagerBalance);
      yieldManager.getTargetReserveDeficit.mockResolvedValueOnce(targetReserveDeficit);
      yieldManager.safeWithdrawFromYieldProvider.mockResolvedValueOnce(createTransactionReceipt("0xwithdraw"));
      yieldManager.fundYieldProvider.mockResolvedValueOnce(createTransactionReceipt("0xtransfer"));
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleRebalance(req: unknown): Promise<void>;
        }
      )._handleRebalance(createRebalanceRequirement(RebalanceDirection.NONE, 0n));

      // Assert
      expect(yieldManager.getBalance).toHaveBeenCalledTimes(1);
      expect(yieldManager.getTargetReserveDeficit).toHaveBeenCalledTimes(1);
      expect(yieldManager.safeWithdrawFromYieldProvider).toHaveBeenCalledWith(YIELD_PROVIDER, targetReserveDeficit);
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledWith(YIELD_PROVIDER, expect.any(Object));
      expect(yieldManager.fundYieldProvider).toHaveBeenCalledWith(YIELD_PROVIDER, yieldManagerBalance);
      expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledWith(YIELD_PROVIDER, expect.any(Object));
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);
    });
  });

  describe("_handleStakingRebalance", () => {
    it("calls rebalance functions and reports yield", async () => {
      // Arrange
      const processor = createProcessor();
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleStakingRebalance(amount: bigint): Promise<void>;
        }
      )._handleStakingRebalance(18n);

      // Assert
      expect(yieldExtension.transferFundsForNativeYield).toHaveBeenCalledWith(18n);
      expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.STAKE, Number(18n));
      expect(yieldManager.fundYieldProvider).toHaveBeenCalledWith(YIELD_PROVIDER, 18n);
      expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledTimes(1);
      const transferResult = metricsRecorder.recordTransferFundsMetrics.mock.calls[0][1];
      expect(transferResult.isOk()).toBe(true);
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);

      reportYieldSpy.mockRestore();
    });

    it("tolerates fundYieldProvider failure", async () => {
      // Arrange
      yieldManager.fundYieldProvider.mockRejectedValueOnce(new Error("fund fail"));
      const processor = createProcessor();
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleStakingRebalance(amount: bigint): Promise<void>;
        }
      )._handleStakingRebalance(11n);

      // Assert
      expect(yieldExtension.transferFundsForNativeYield).toHaveBeenCalledWith(11n);
      expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.STAKE, Number(11n));
      expect(metricsRecorder.recordTransferFundsMetrics).toHaveBeenCalledTimes(1);
      const result = metricsRecorder.recordTransferFundsMetrics.mock.calls[0][1];
      expect(result.isErr()).toBe(true);
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);

      reportYieldSpy.mockRestore();
    });

    it("skips fundYieldProvider when transferFundsForNativeYield fails", async () => {
      // Arrange
      yieldExtension.transferFundsForNativeYield.mockRejectedValueOnce(new Error("transfer fail"));
      const processor = createProcessor();
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleStakingRebalance(amount: bigint): Promise<void>;
        }
      )._handleStakingRebalance(9n);

      // Assert
      expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
      expect(yieldManager.fundYieldProvider).not.toHaveBeenCalled();
      expect(metricsRecorder.recordTransferFundsMetrics).not.toHaveBeenCalled();
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);

      reportYieldSpy.mockRestore();
    });
  });

  describe("_handleUnstakingRebalance", () => {
    it("reports yield before withdrawing when shouldReportYield is true", async () => {
      // Arrange
      const processor = createProcessor();
      const reportYieldSpy = jest
        .spyOn(processor as unknown as { _handleReportYield(): Promise<unknown> }, "_handleReportYield")
        .mockResolvedValue(undefined);

      // Act
      await (
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        }
      )._handleUnstakingRebalance(15n, true);

      // Assert
      expect(reportYieldSpy).toHaveBeenCalledTimes(1);
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
    });

    it("skips yield reporting when shouldReportYield is false", async () => {
      // Arrange
      const processor = createProcessor();
      const reportYieldSpy = jest.spyOn(
        processor as unknown as { _handleReportYield(): Promise<unknown> },
        "_handleReportYield",
      );

      // Act
      await (
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        }
      )._handleUnstakingRebalance(7n, false);

      // Assert
      expect(reportYieldSpy).not.toHaveBeenCalled();
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
    });

    it("tolerates safeAddToWithdrawalReserveIfAboveThreshold failure", async () => {
      // Arrange
      const error = new Error("withdrawal failed");
      yieldManager.safeAddToWithdrawalReserveIfAboveThreshold.mockRejectedValueOnce(error);
      const processor = createProcessor();

      // Act
      await (
        processor as unknown as {
          _handleUnstakingRebalance(amount: bigint, shouldReportYield: boolean): Promise<void>;
        }
      )._handleUnstakingRebalance(20n, false);

      // Assert
      expect(metricsRecorder.recordSafeWithdrawalMetrics).toHaveBeenCalledTimes(1);
      const result = metricsRecorder.recordSafeWithdrawalMetrics.mock.calls[0][1];
      expect(result.isErr()).toBe(true);
    });
  });

  describe("_handleReportYield", () => {
    it("reports yield when condition is met", async () => {
      // Arrange
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;
      const shouldReportYieldSpy = jest
        .spyOn(processor as unknown as { _shouldReportYield(): Promise<boolean> }, "_shouldReportYield")
        .mockResolvedValue(true);

      // Act
      await (
        processor as unknown as {
          _handleReportYield(): Promise<void>;
        }
      )._handleReportYield();

      // Assert
      expect(shouldReportYieldSpy).toHaveBeenCalledTimes(1);
      expect(yieldManager.reportYield).toHaveBeenCalledWith(YIELD_PROVIDER, L2_RECIPIENT);
      expect(metricsRecorder.recordReportYieldMetrics).toHaveBeenCalledWith(
        YIELD_PROVIDER,
        expect.objectContaining({ isOk: expect.any(Function) }),
      );
      expect(logger.info).toHaveBeenCalledWith("_handleReportYield: yield report succeeded");

      shouldReportYieldSpy.mockRestore();
    });

    it("skips yield reporting when condition is not met", async () => {
      // Arrange
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;
      const shouldReportYieldSpy = jest
        .spyOn(processor as unknown as { _shouldReportYield(): Promise<boolean> }, "_shouldReportYield")
        .mockResolvedValue(false);

      // Act
      await (
        processor as unknown as {
          _handleReportYield(): Promise<void>;
        }
      )._handleReportYield();

      // Assert
      expect(shouldReportYieldSpy).toHaveBeenCalledTimes(1);
      expect(yieldManager.reportYield).not.toHaveBeenCalled();
      expect(metricsRecorder.recordReportYieldMetrics).not.toHaveBeenCalled();

      shouldReportYieldSpy.mockRestore();
    });

    it("tolerates reportYield failure", async () => {
      // Arrange
      yieldManager.reportYield.mockRejectedValueOnce(new Error("yield fail"));
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;
      const shouldReportYieldSpy = jest
        .spyOn(processor as unknown as { _shouldReportYield(): Promise<boolean> }, "_shouldReportYield")
        .mockResolvedValue(true);

      // Act
      await (
        processor as unknown as {
          _handleReportYield(): Promise<void>;
        }
      )._handleReportYield();

      // Assert
      expect(shouldReportYieldSpy).toHaveBeenCalledTimes(1);
      expect(yieldManager.reportYield).toHaveBeenCalledWith(YIELD_PROVIDER, L2_RECIPIENT);
      expect(metricsRecorder.recordReportYieldMetrics).not.toHaveBeenCalled();

      shouldReportYieldSpy.mockRestore();
    });
  });

  describe("_handleSubmitLatestVaultReport", () => {
    it("submits vault report successfully", async () => {
      // Arrange
      vaultHubClient.isReportFresh.mockResolvedValueOnce(false);
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;

      // Act
      await (
        processor as unknown as {
          _handleSubmitLatestVaultReport(): Promise<void>;
        }
      )._handleSubmitLatestVaultReport();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsUpdater.incrementLidoVaultAccountingReport).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(logger.info).toHaveBeenCalledWith("_handleSubmitLatestVaultReport - Vault report submission succeeded");
      expect(yieldManager.reportYield).not.toHaveBeenCalled();
      expect(metricsRecorder.recordReportYieldMetrics).not.toHaveBeenCalled();
    });

    it("handles vault submission failure gracefully", async () => {
      // Arrange
      vaultHubClient.isReportFresh.mockResolvedValueOnce(false);
      lidoReportClient.submitLatestVaultReport.mockRejectedValueOnce(new Error("vault fail"));
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;

      // Act
      await (
        processor as unknown as {
          _handleSubmitLatestVaultReport(): Promise<void>;
        }
      )._handleSubmitLatestVaultReport();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(yieldManager.reportYield).not.toHaveBeenCalled();
      expect(metricsRecorder.recordReportYieldMetrics).not.toHaveBeenCalled();
    });

    it("skips submission when report is fresh", async () => {
      // Arrange
      vaultHubClient.isReportFresh.mockResolvedValueOnce(true);
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;

      // Act
      await (
        processor as unknown as {
          _handleSubmitLatestVaultReport(): Promise<void>;
        }
      )._handleSubmitLatestVaultReport();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
      expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith(
        "_handleSubmitLatestVaultReport - Skipping vault report submission (report is fresh)",
      );
    });

    it("proceeds with submission when isReportFresh check fails", async () => {
      // Arrange
      vaultHubClient.isReportFresh.mockRejectedValueOnce(new Error("check failed"));
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;

      // Act
      await (
        processor as unknown as {
          _handleSubmitLatestVaultReport(): Promise<void>;
        }
      )._handleSubmitLatestVaultReport();

      // Assert
      expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining("Failed to check if report is fresh, proceeding with submission attempt"),
      );
      expect(lidoReportClient.getLatestSubmitVaultReportParams).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalled();
    });

    it("skips vault report submission when shouldSubmitVaultReport is false", async () => {
      // Arrange
      const processor = createProcessor(false, true, true);
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;

      // Act
      await (
        processor as unknown as {
          _handleSubmitLatestVaultReport(): Promise<void>;
        }
      )._handleSubmitLatestVaultReport();

      // Assert
      expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
      expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith(
        "_handleSubmitLatestVaultReport - Skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)",
      );
      expect(yieldManager.reportYield).not.toHaveBeenCalled();
      expect(metricsRecorder.recordReportYieldMetrics).not.toHaveBeenCalled();
    });
  });

  describe("_shouldReportYield", () => {
    it("returns true when negative yield diff threshold is met", async () => {
      // Arrange
      const onStateNegativeYield = ONE_ETH;
      const peekedNegativeYield = 2n * ONE_ETH + HALF_ETH;
      const yieldReport = createYieldReport(HALF_ETH, peekedNegativeYield, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_BELOW_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      yieldManager.getYieldProviderData.mockResolvedValue(createYieldProviderData(onStateNegativeYield));
      const processor = createProcessor();

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(true);
      expect(yieldManager.getYieldProviderData).toHaveBeenCalledWith(YIELD_PROVIDER);
    });

    it("returns false when negative yield diff is below threshold", async () => {
      // Arrange
      const onStateNegativeYield = ONE_ETH;
      const peekedNegativeYield = ONE_ETH + HALF_ETH;
      const yieldReport = createYieldReport(HALF_ETH, peekedNegativeYield, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_BELOW_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      yieldManager.getYieldProviderData.mockResolvedValue(createYieldProviderData(onStateNegativeYield));
      const processor = createProcessor();
      (processor as any).cycleCount = 1;

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(false);
    });

    it("returns false when neither threshold is met", async () => {
      // Arrange
      const yieldReport = createYieldReport(HALF_ETH, 0n, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_BELOW_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      const processor = createProcessor();
      (processor as any).cycleCount = 1;

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(false);
    });

    it("treats undefined yieldReport as zero values", async () => {
      // Arrange
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_BELOW_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(undefined);
      const processor = createProcessor();
      (processor as any).cycleCount = 1;

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(false);
    });

    it("returns false when both values are zero", async () => {
      // Arrange
      const yieldReport = createYieldReport(0n, 0n, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(0n);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      const processor = createProcessor();
      (processor as any).cycleCount = 1;

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(false);
    });

    it("logs yield report details", async () => {
      // Arrange
      const yieldReport = createYieldReport(TWO_ETH, 0n, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_ABOVE_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      const processor = createProcessor();
      (processor as any).cycleCount = 12;

      // Act
      await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("_shouldReportYield - shouldReportYield=true"));
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining('settleableLidoFees="600000000000000000"'));
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining('"yieldAmount":"2000000000000000000"'));
    });

    it("sets metrics when reads succeed", async () => {
      // Arrange
      const yieldReport = createYieldReport(TWO_ETH, ONE_ETH, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_ABOVE_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;

      // Act
      await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(metricsUpdater.setLastPeekedNegativeYieldReport).toHaveBeenCalledWith(VAULT_ADDRESS, Number(ONE_ETH));
      expect(metricsUpdater.setLastPeekedPositiveYieldReport).toHaveBeenCalledWith(VAULT_ADDRESS, Number(TWO_ETH));
      expect(metricsUpdater.setLastSettleableLidoFees).toHaveBeenCalledWith(
        VAULT_ADDRESS,
        Number(SETTLEABLE_FEES_ABOVE_THRESHOLD),
      );
    });

    it("skips yield report metrics when yieldReport is undefined", async () => {
      // Arrange
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_BELOW_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(undefined);
      const processor = createProcessor();
      (processor as unknown as { vault: Address }).vault = VAULT_ADDRESS;

      // Act
      await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(metricsUpdater.setLastPeekedNegativeYieldReport).not.toHaveBeenCalled();
      expect(metricsUpdater.setLastPeekedPositiveYieldReport).not.toHaveBeenCalled();
      expect(metricsUpdater.setLastSettleableLidoFees).toHaveBeenCalledWith(
        VAULT_ADDRESS,
        Number(SETTLEABLE_FEES_BELOW_THRESHOLD),
      );
    });

    it("returns true when cycle count is divisible by cyclesPerYieldReport", async () => {
      // Arrange
      const yieldReport = createYieldReport(HALF_ETH, 0n, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_BELOW_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      yieldManager.getLidoStakingVaultAddress.mockResolvedValue(VAULT_ADDRESS);
      const processor = createProcessor();
      (processor as any).vault = VAULT_ADDRESS;
      (processor as any).cycleCount = 24;

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(true);
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("cycleBasedReportingDue=true"));
    });

    it("returns false when shouldReportYield config is false even if thresholds are met", async () => {
      // Arrange
      const onStateNegativeYield = 0n;
      const peekedNegativeYield = TWO_ETH;
      const yieldReport = createYieldReport(0n, peekedNegativeYield, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_ABOVE_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      yieldManager.getYieldProviderData.mockResolvedValue(createYieldProviderData(onStateNegativeYield));
      const processor = createProcessor(true, false);
      (processor as any).vault = VAULT_ADDRESS;

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(false);
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("_shouldReportYield - shouldReportYield=false"));
    });

    it("returns false when cycle count is not divisible and thresholds not met", async () => {
      // Arrange
      const yieldReport = createYieldReport(HALF_ETH, 0n, YIELD_PROVIDER);
      vaultHubClient.settleableLidoFeesValue.mockResolvedValue(SETTLEABLE_FEES_BELOW_THRESHOLD);
      yieldManager.peekYieldReport.mockResolvedValue(yieldReport);
      yieldManager.getLidoStakingVaultAddress.mockResolvedValue(VAULT_ADDRESS);
      const processor = createProcessor();
      (processor as any).vault = VAULT_ADDRESS;
      (processor as any).cycleCount = 11;

      // Act
      const result = await (processor as unknown as { _shouldReportYield(): Promise<boolean> })._shouldReportYield();

      // Assert
      expect(result).toBe(false);
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("cycleBasedReportingDue=false"));
    });
  });
});
