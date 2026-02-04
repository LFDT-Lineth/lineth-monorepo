import { jest, describe, it, expect, beforeEach } from "@jest/globals";
import { ok, err } from "neverthrow";
import type { TransactionReceipt, Address, PublicClient } from "viem";
import type { IBlockchainClient } from "@consensys/linea-shared-utils";
import type { IYieldManager } from "../../../core/clients/contracts/IYieldManager.js";
import type { IVaultHub } from "../../../core/clients/contracts/IVaultHub.js";
import { createLoggerMock, createMetricsUpdaterMock } from "../../../__tests__/helpers/index.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import { DashboardContractClient } from "../../../clients/contracts/DashboardContractClient.js";
import { OperationModeMetricsRecorder } from "../OperationModeMetricsRecorder.js";

const ONE_GWEI = 1_000_000_000n;

const YIELD_PROVIDER = "0xyieldprovider" as Address;
const ALTERNATE_YIELD_PROVIDER = "0xalternate" as Address;
const VAULT_ADDRESS = "0xvault" as Address;
const DASHBOARD_ADDRESS = "0xdashboard" as Address;

const toWei = (gwei: number): bigint => BigInt(gwei) * ONE_GWEI;

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

const createTransactionReceipt = (): TransactionReceipt => ({}) as TransactionReceipt;

jest.mock("../../../clients/contracts/DashboardContractClient.js", () => ({
  DashboardContractClient: {
    getOrCreate: jest.fn(),
    initialize: jest.fn(),
  },
}));

describe("OperationModeMetricsRecorder", () => {
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

  describe("recordProgressOssificationMetrics", () => {
    it("skips metrics recording when receipt result is error", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const error = new Error("boom");

      // Act
      await recorder.recordProgressOssificationMetrics(YIELD_PROVIDER, err(error));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordProgressOssificationMetrics - transaction receipt result is error, skipping metrics recording",
        { error },
      );
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
      expect(metricsUpdater.addNodeOperatorFeesPaid).not.toHaveBeenCalled();
    });

    it("skips metrics recording when receipt is undefined", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);

      // Act
      await recorder.recordProgressOssificationMetrics(
        YIELD_PROVIDER,
        ok<TransactionReceipt | undefined, Error>(undefined),
      );

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordProgressOssificationMetrics - receipt is undefined, skipping metrics recording",
      );
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
      expect(metricsUpdater.addNodeOperatorFeesPaid).not.toHaveBeenCalled();
    });

    it("records node operator, lido fee, and liability metrics with non-zero values", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(VAULT_ADDRESS);
      yieldManagerClient.getLidoDashboardAddress.mockResolvedValueOnce(DASHBOARD_ADDRESS);
      dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt.mockReturnValueOnce(toWei(5));
      vaultHubClient.getLidoFeePaymentFromTxReceipt.mockReturnValueOnce(toWei(3));
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(7));

      // Act
      await recorder.recordProgressOssificationMetrics(
        YIELD_PROVIDER,
        ok<TransactionReceipt | undefined, Error>(receipt),
      );

      // Assert
      expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(DASHBOARD_ADDRESS);
      expect(dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt).toHaveBeenCalledWith(receipt);
      expect(metricsUpdater.addNodeOperatorFeesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 5);
      expect(metricsUpdater.addLidoFeesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 3);
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 7);
    });

    it("records metrics when extracted values are all zero", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(VAULT_ADDRESS);
      yieldManagerClient.getLidoDashboardAddress.mockResolvedValueOnce(DASHBOARD_ADDRESS);
      dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt.mockReturnValueOnce(0n);
      vaultHubClient.getLidoFeePaymentFromTxReceipt.mockReturnValueOnce(0n);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(0n);

      // Act
      await recorder.recordProgressOssificationMetrics(
        YIELD_PROVIDER,
        ok<TransactionReceipt | undefined, Error>(receipt),
      );

      // Assert
      expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(DASHBOARD_ADDRESS);
      expect(dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt).toHaveBeenCalledWith(receipt);
      expect(metricsUpdater.addNodeOperatorFeesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 0);
      expect(metricsUpdater.addLidoFeesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 0);
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 0);
    });
  });

  describe("recordReportYieldMetrics", () => {
    it("skips metrics recording when receipt result is error", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const error = new Error("boom");

      // Act
      await recorder.recordReportYieldMetrics(YIELD_PROVIDER, err(error));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordReportYieldMetrics - transaction receipt result is error, skipping metrics recording",
        { error },
      );
      expect(metricsUpdater.incrementReportYield).not.toHaveBeenCalled();
    });

    it("skips metrics recording when receipt is undefined", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);

      // Act
      await recorder.recordReportYieldMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(undefined));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordReportYieldMetrics - receipt is undefined, skipping metrics recording",
      );
      expect(metricsUpdater.incrementReportYield).not.toHaveBeenCalled();
    });

    it("skips metrics recording when yield report is not found in receipt", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getYieldReportFromTxReceipt.mockReturnValueOnce(undefined);

      // Act
      await recorder.recordReportYieldMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(receipt));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordReportYieldMetrics - yield report not found in receipt, skipping metrics recording",
      );
      expect(metricsUpdater.incrementReportYield).not.toHaveBeenCalled();
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
    });

    it("records yield metrics and payouts when yield report is available", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getYieldReportFromTxReceipt.mockReturnValueOnce({
        yieldAmount: toWei(11),
        outstandingNegativeYield: toWei(2),
        yieldProvider: ALTERNATE_YIELD_PROVIDER,
      });
      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(VAULT_ADDRESS);
      yieldManagerClient.getLidoDashboardAddress.mockResolvedValueOnce(DASHBOARD_ADDRESS);
      dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt.mockReturnValueOnce(toWei(4));
      vaultHubClient.getLidoFeePaymentFromTxReceipt.mockReturnValueOnce(toWei(6));
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(8));

      // Act
      await recorder.recordReportYieldMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(receipt));

      // Assert
      expect(DashboardContractClient.getOrCreate).toHaveBeenCalledWith(DASHBOARD_ADDRESS);
      expect(dashboardClientInstance.getNodeOperatorFeesPaidFromTxReceipt).toHaveBeenCalledWith(receipt);
      expect(yieldManagerClient.getLidoStakingVaultAddress).toHaveBeenCalledWith(ALTERNATE_YIELD_PROVIDER);
      expect(metricsUpdater.incrementReportYield).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsUpdater.addNodeOperatorFeesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 4);
      expect(metricsUpdater.addLidoFeesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 6);
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 8);
    });
  });

  describe("recordSafeWithdrawalMetrics", () => {
    it("skips metrics recording when receipt result is error", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const error = new Error("boom");

      // Act
      await recorder.recordSafeWithdrawalMetrics(YIELD_PROVIDER, err(error));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordSafeWithdrawalMetrics - transaction receipt result is error, skipping metrics recording",
        { error },
      );
      expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
      expect(yieldManagerClient.getWithdrawalEventFromTxReceipt).not.toHaveBeenCalled();
    });

    it("skips metrics recording when receipt is undefined", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);

      // Act
      await recorder.recordSafeWithdrawalMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(undefined));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordSafeWithdrawalMetrics - receipt is undefined, skipping metrics recording",
      );
      expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
      expect(yieldManagerClient.getWithdrawalEventFromTxReceipt).not.toHaveBeenCalled();
    });

    it("records rebalance and liabilities when withdrawal event is present", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getWithdrawalEventFromTxReceipt.mockReturnValueOnce({
        reserveIncrementAmount: toWei(9),
        yieldProvider: YIELD_PROVIDER,
      });
      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(VAULT_ADDRESS);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(5));

      // Act
      await recorder.recordSafeWithdrawalMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(receipt));

      // Assert
      expect(metricsUpdater.recordRebalance).toHaveBeenCalledWith(RebalanceDirection.UNSTAKE, 9);
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 5);
    });

    it("skips metrics recording when withdrawal event is not found in receipt", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getWithdrawalEventFromTxReceipt.mockReturnValueOnce(undefined);

      // Act
      await recorder.recordSafeWithdrawalMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(receipt));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordSafeWithdrawalMetrics - withdrawal event not found in receipt, skipping metrics recording",
      );
      expect(metricsUpdater.recordRebalance).not.toHaveBeenCalled();
    });
  });

  describe("recordTransferFundsMetrics", () => {
    it("skips metrics recording when receipt result is error", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const error = new Error("boom");

      // Act
      await recorder.recordTransferFundsMetrics(YIELD_PROVIDER, err(error));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordTransferFundsMetrics - transaction receipt result is error, skipping metrics recording",
        { error },
      );
      expect(metricsUpdater.addLiabilitiesPaid).not.toHaveBeenCalled();
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
    });

    it("skips metrics recording when receipt is undefined", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);

      // Act
      await recorder.recordTransferFundsMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(undefined));

      // Assert
      expect(logger.warn).toHaveBeenCalledWith(
        "recordTransferFundsMetrics - receipt is undefined, skipping metrics recording",
      );
      expect(metricsUpdater.addLiabilitiesPaid).not.toHaveBeenCalled();
      expect(yieldManagerClient.getLidoStakingVaultAddress).not.toHaveBeenCalled();
    });

    it("records liabilities when liability payment exists", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(VAULT_ADDRESS);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(toWei(12));

      // Act
      await recorder.recordTransferFundsMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(receipt));

      // Assert
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 12);
    });

    it("records liabilities when payment is zero", async () => {
      // Arrange
      const logger = createLoggerMock();
      const metricsUpdater = createMetricsUpdaterMock();
      const yieldManagerClient = createYieldManagerMock();
      const vaultHubClient = createVaultHubMock();
      const recorder = new OperationModeMetricsRecorder(logger, metricsUpdater, yieldManagerClient, vaultHubClient);
      const receipt = createTransactionReceipt();

      yieldManagerClient.getLidoStakingVaultAddress.mockResolvedValueOnce(VAULT_ADDRESS);
      vaultHubClient.getLiabilityPaymentFromTxReceipt.mockReturnValueOnce(0n);

      // Act
      await recorder.recordTransferFundsMetrics(YIELD_PROVIDER, ok<TransactionReceipt | undefined, Error>(receipt));

      // Assert
      expect(metricsUpdater.addLiabilitiesPaid).toHaveBeenCalledWith(VAULT_ADDRESS, 0);
    });
  });
});
