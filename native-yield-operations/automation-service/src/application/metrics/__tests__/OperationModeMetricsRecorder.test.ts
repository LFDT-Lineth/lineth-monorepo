import { jest } from "@jest/globals";
import { ok, err } from "neverthrow";
import { OperationModeMetricsRecorder } from "../OperationModeMetricsRecorder.js";
import type { TransactionReceipt, Address, PublicClient } from "viem";
import type { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IYieldManager } from "../../../core/clients/contracts/IYieldManager.js";
import type { IVaultHub } from "../../../core/clients/contracts/IVaultHub.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import { DashboardContractClient } from "../../../clients/contracts/DashboardContractClient.js";

const ONE_GWEI = 1_000_000_000n;
const toWei = (gwei: number): bigint => BigInt(gwei) * ONE_GWEI;

const createLoggerMock = (): ILogger => ({
  name: "test-logger",
  info: jest.fn(),
  error: jest.fn(),
  warn: jest.fn(),
  debug: jest.fn(),
});

const createMetricsUpdaterMock = (): jest.Mocked<INativeYieldAutomationMetricsUpdater> =>
  ({
    recordRebalance: jest.fn(),
    recordOperationModeDuration: jest.fn(),
    incrementReportYield: jest.fn(),
    addReportedYieldAmount: jest.fn(),
    setLastPeekedNegativeYieldReport: jest.fn(async () => undefined),
    setLastPeekedPositiveYieldReport: jest.fn(async () => undefined),
    setLastPeekUnpaidLidoProtocolFees: jest.fn(async () => undefined),
    addNodeOperatorFeesPaid: jest.fn(),
    addLiabilitiesPaid: jest.fn(),
    addLidoFeesPaid: jest.fn(),
    incrementLidoVaultAccountingReport: jest.fn(),
    incrementOperationModeExecution: jest.fn(),
    incrementOperationModeTrigger: jest.fn(),
    addValidatorPartialUnstakeAmount: jest.fn(),
    incrementValidatorExit: jest.fn(),
  }) as unknown as jest.Mocked<INativeYieldAutomationMetricsUpdater>;

const createYieldManagerMock = () =>
  ({
    getLidoStakingVaultAddress: jest.fn(),
    getLidoDashboardAddress: jest.fn(),
    getYieldReportFromTxReceipt: jest.fn(),
    getWithdrawalEventFromTxReceipt: jest.fn(),
  }) as unknown as jest.Mocked<IYieldManager<TransactionReceipt>>;

const createVaultHubMock = () =>
  ({
    getLiabilityPaymentFromTxReceipt: jest.fn(),
    getLidoFeePaymentFromTxReceipt: jest.fn(),
  }) as unknown as jest.Mocked<IVaultHub<TransactionReceipt>>;

const createBlockchainClientMock = () =>
  ({
    getBlockchainClient: jest.fn(),
  }) as unknown as jest.Mocked<IBlockchainClient<PublicClient, TransactionReceipt>>;

jest.mock("../../../clients/contracts/DashboardContractClient.js", () => ({
  DashboardContractClient: {
    getOrCreate: jest.fn(),
    initialize: jest.fn(),
  },
}));

describe("OperationModeMetricsRecorder", () => {
  const yieldProvider = "0xyieldprovider" as Address;
  const alternateYieldProvider = "0xalternate" as Address;
  const vaultAddress = "0xvault" as Address;
  const dashboardAddress = "0xdashboard" as Address;
  const receipt = {} as TransactionReceipt;

  let dashboardClientInstance: jest.Mocked<DashboardContractClient>;

  beforeEach(() => {
    jest.clearAllMocks();
    dashboardClientInstance = {
      getNodeOperatorFeesPaidFromTxReceipt: jest.fn(),
      getAddress: jest.fn(),
      getContract: jest.fn(),
    } as unknown as jest.Mocked<DashboardContractClient>;
    const blockchainClient = createBlockchainClientMock();
    const logger = createLoggerMock();
    DashboardContractClient.initialize(blockchainClient, logger);
    (
      DashboardContractClient.getOrCreate as jest.MockedFunction<typeof DashboardContractClient.getOrCreate>
    ).mockReturnValue(dashboardClientInstance);
  });

  const setupRecorder = () => {
    const logger = createLoggerMock();
    const metricsUpdater = createMetricsUpdaterMock();
    const yieldManagerClient = createYieldManagerMock();
    const vaultHubClient = createVaultHubMock();

    const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);

    return { recorder, metricsUpdater, yieldManagerClient, vaultHubClient };
  };

  describe("recordProgressOssificationMetrics", () => {
    it("does nothing when receipt is an error", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      await recorder.recordProgressOssificationMetrics(yieldProvider, err(new Error("boom")));

      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
      expect(metricsUpdater.addNodeOperatorFeesPaid).not.toHaveBeenCalled();
    });

    it("does nothing when receipt is undefined", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      await recorder.recordProgressOssificationMetrics(
        yieldProvider,
        ok<TransactionReceipt | undefined, Error>(undefined),
      );

      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
      expect(metricsUpdater.addNodeOperatorFeesPaid).not.toHaveBeenCalled();
    });

    it("records node operator, lido fee, and liability metrics when values are non-zero", async () => {
      const { recorder, metricsUpdater, yieldManagerClient, vaultHubClient } = setupRecorder();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(vaultAddress);
      yieldManagerClient.getLidoDashboardAddress.mockResolvedValueOnce(dashboardAddress);
      dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt.mockReturnValueOnce(toWei(5));
      vaultHubClient.getLidoFeePaymentFromTxReceipt.mockReturnValueOnce(toWei(3));
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(7));

      await recorder.recordProgressOssificationMetrics(
        yieldProvider,
        ok<TransactionReceipt | undefined, Error>(receipt),
      );

      expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(dashboardAddress);
      expect(dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt).toHaveBeenCalledWith(receipt);
      expect(metricsUpdater.addNodeOperatorFeesPaid).toHaveBeenCalledWith(vaultAddress, 5);
      expect(metricsUpdater.addLidoFeesPaid).toHaveBeenCalledWith(vaultAddress, 3);
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(vaultAddress, 7);
    });

    it("skips metric updates when all extracted values are zero", async () => {
      const { recorder, metricsUpdater, yieldManagerClient, vaultHubClient } = setupRecorder();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(vaultAddress);
      yieldManagerClient.getLidoDashboardAddress.mockResolvedValueOnce(dashboardAddress);
      dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt.mockReturnValueOnce(0n);
      vaultHubClient.getLidoFeePaymentFromTxReceipt.mockReturnValueOnce(0n);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(0n);

      await recorder.recordProgressOssificationMetrics(
        yieldProvider,
        ok<TransactionReceipt | undefined, Error>(receipt),
      );

      expect(metricsUpdater.addNodeOperatorFeesPaid).not.toHaveBeenCalled();
      expect(metricsUpdater.addLidoFeesPaid).not.toHaveBeenCalled();
      expect(metricsUpdater.addLiabilitiesPaid).not.toHaveBeenCalled();
    });
  });

  describe("recordReportYieldMetrics", () => {
    it("does nothing when receipt result is error", async () => {
      const { recorder, metricsUpdater } = setupRecorder();

      await recorder.recordReportYieldMetrics(yieldProvider, err(new Error("boom")));

      expect(metricsUpdater.incrementReportYield).not.toHaveBeenCalled();
    });

    it("does nothing when receipt is undefined", async () => {
      const { recorder, metricsUpdater } = setupRecorder();

      await recorder.recordReportYieldMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(undefined));

      expect(metricsUpdater.incrementReportYield).not.toHaveBeenCalled();
    });

    it("does nothing when yield report cannot be parsed", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      yieldManagerClient.getYieldReportFromTxReceipt.mockReturnValueOnce(undefined);

      await recorder.recordReportYieldMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(receipt));

      expect(metricsUpdater.incrementReportYield).not.toHaveBeenCalled();
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
    });

    it("records yield metrics and payouts when report is available", async () => {
      const { recorder, metricsUpdater, yieldManagerClient, vaultHubClient } = setupRecorder();

      yieldManagerClient.getYieldReportFromTxReceipt.mockReturnValueOnce({
        yieldAmount: toWei(11),
        outstandingNegativeYield: toWei(2),
        yieldProvider: alternateYieldProvider,
      });
      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(vaultAddress);
      yieldManagerClient.getLidoDashboardAddress.mockResolvedValueOnce(dashboardAddress);
      dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt.mockReturnValueOnce(toWei(4));
      vaultHubClient.getLidoFeePaymentFromTxReceipt.mockReturnValueOnce(toWei(6));
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(8));

      await recorder.recordReportYieldMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(receipt));

      expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(dashboardAddress);
      expect(dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt).toHaveBeenCalledWith(receipt);
      expect(yieldManagerClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(alternateYieldProvider);
      expect(metricsUpdater.incrementReportYield).toHaveBeenCalledWith(vaultAddress);
      expect(metricsUpdater.addReportedYieldAmount).toHaveBeenCalledWith(vaultAddress, 11);
      expect(metricsUpdater.addNodeOperatorFeesPaid).toHaveBeenCalledWith(vaultAddress, 4);
      expect(metricsUpdater.addLidoFeesPaid).toHaveBeenCalledWith(vaultAddress, 6);
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(vaultAddress, 8);
    });
  });

  describe("recordSafeWithdrawalMetrics", () => {
    it("does nothing when receipt result is error", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      await recorder.recordSafeWithdrawalMetrics(yieldProvider, err(new Error("boom")));

      expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
      expect(yieldManagerClient.getWithdrawalEventFromTxReceipt).not.toHaveBeenCalled();
    });

    it("does nothing when receipt is undefined", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      await recorder.recordSafeWithdrawalMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(undefined));

      expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
      expect(yieldManagerClient.getWithdrawalEventFromTxReceipt).not.toHaveBeenCalled();
    });

    it("records rebalance and liabilities when withdrawal event is present", async () => {
      const { recorder, metricsUpdater, yieldManagerClient, vaultHubClient } = setupRecorder();

      yieldManagerClient.getWithdrawalEventFromTxReceipt.mockReturnValueOnce({
        reserveIncrementAmount: toWei(9),
        yieldProvider,
      });
      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(vaultAddress);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(5));

      await recorder.recordSafeWithdrawalMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(receipt));

      expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.UNSTAKE, 9);
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(vaultAddress, 5);
    });

    it("does nothing when no withdrawal event is found", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      yieldManagerClient.getWithdrawalEventFromTxReceipt.mockReturnValueOnce(undefined);

      await recorder.recordSafeWithdrawalMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(receipt));

      expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
    });
  });

  describe("recordTransferFundsMetrics", () => {
    it("does nothing when receipt result is error", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      await recorder.recordTransferFundsMetrics(yieldProvider, err(new Error("boom")));

      expect(metricsUpdater.addLiabilitiesPaid).not.toHaveBeenCalled();
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
    });

    it("does nothing when receipt is undefined", async () => {
      const { recorder, metricsUpdater, yieldManagerClient } = setupRecorder();

      await recorder.recordTransferFundsMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(undefined));

      expect(metricsUpdater.addLiabilitiesPaid).not.toHaveBeenCalled();
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
    });

    it("records liabilities when payment exists", async () => {
      const { recorder, metricsUpdater, yieldManagerClient, vaultHubClient } = setupRecorder();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(vaultAddress);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(12));

      await recorder.recordTransferFundsMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(receipt));

      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(vaultAddress, 12);
    });

    it("skips liabilities when payment is zero", async () => {
      const { recorder, metricsUpdater, yieldManagerClient, vaultHubClient } = setupRecorder();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(vaultAddress);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(0n);

      await recorder.recordTransferFundsMetrics(yieldProvider, ok<TransactionReceipt | undefined, Error>(receipt));

      expect(metricsUpdater.addLiabilitiesPaid).not.toHaveBeenCalled();
    });
  });
});
