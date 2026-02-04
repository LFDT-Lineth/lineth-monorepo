import { jest } from "@jest/globals";
import { IMetricsService } from "@consensys/linea-shared-utils";
import { NativeYieldAutomationMetricsUpdater } from "../NativeYieldAutomationMetricsUpdater.js";
import {
  LineaNativeYieldAutomationServiceMetrics,
  OperationModeExecutionStatus,
} from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import { OperationMode } from "../../../core/enums/OperationModeEnums.js";
import { Address, Hex } from "viem";

// Test data constants
const VALIDATOR_PUBKEY = "0xvalidator" as Hex;
const VAULT_ADDRESS = "0xvault" as Address;
const CONTRACT_ADDRESS = "0x1234567890123456789012345678901234567890" as Address;
const STANDARD_VALIDATOR_AMOUNT_GWEI = 32000000000;
const SAMPLE_AMOUNT_GWEI = 1000;
const SAMPLE_EPOCH = 60001;
const SAMPLE_SLOT = 123456;
const SAMPLE_TIMESTAMP = 1704067200;
const RAW_REVERT_DATA = "0xf2ed496c000000000000000000000000000000000000000000000025dffc6dedca6c668800000000000000000000000000000000000000000000000ac3b0cfe3a6daf2d1";
const ERROR_NAME = "ExceedsWithdrawable";

// Factory functions
const createMetricsServiceMock = (): jest.Mocked<IMetricsService<LineaNativeYieldAutomationServiceMetrics>> =>
  ({
    getRegistry: jest.fn(),
    createCounter: jest.fn(),
    createGauge: jest.fn(),
    setGauge: jest.fn(),
    incrementCounter: jest.fn(),
    incrementGauge: jest.fn(),
    decrementGauge: jest.fn(),
    getGaugeValue: jest.fn(),
    getCounterValue: jest.fn(),
    createHistogram: jest.fn(),
    addValueToHistogram: jest.fn(),
    getHistogramMetricsValues: jest.fn(),
  }) as unknown as jest.Mocked<IMetricsService<LineaNativeYieldAutomationServiceMetrics>>;

describe("NativeYieldAutomationMetricsUpdater", () => {
  describe("constructor", () => {
    it("registers all counter metrics", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();

      // Act
      new NativeYieldAutomationMetricsUpdater(metricsService);

      // Assert
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.RebalanceAmountTotal,
        "Total rebalance amount between L1MessageService and YieldProvider",
        ["direction", "type"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorPartialUnstakeAmountTotal,
        "Total amount partially unstaked per validator",
        ["validator_pubkey"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorExitTotal,
        "Total validator exits initiated by automation",
        ["validator_pubkey"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LidoVaultAccountingReportSubmittedTotal,
        "Accounting reports submitted to Lido per vault",
        ["vault_address"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal,
        "Yield reports submitted to YieldManager per vault",
        ["vault_address"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.NodeOperatorFeesPaidTotal,
        "Node operator fees paid by automation per vault",
        ["vault_address"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LiabilitiesPaidTotal,
        "Liabilities paid by automation per vault",
        ["vault_address"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LidoFeesPaidTotal,
        "Lido fees paid by automation per vault",
        ["vault_address"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
        "Operation mode executions grouped by mode and status",
        ["mode", "status"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.StakingDepositQuotaExceeded,
        "Total number of times the staking deposit quota has been exceeded",
        ["vault_address"],
      );
      expect(metricsService.createCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ContractEstimateGasError,
        "Total number of contract estimateGas errors",
        ["contract_address", "rawRevertData", "errorName"],
      );
    });

    it("registers all gauge metrics", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();

      // Act
      new NativeYieldAutomationMetricsUpdater(metricsService);

      // Assert
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorStakedAmountGwei,
        "Amount staked in a validator in gwei",
        ["pubkey"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedNegativeYieldReport,
        "Outstanding negative yield from the last peeked yield report",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedPositiveYieldReport,
        "Positive yield amount from the last peeked yield report",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastSettleableLidoFees,
        "Settleable Lido protocol fees from the last query",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastVaultReportTimestamp,
        "Timestamp from the latest vault report",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.YieldReportedCumulative,
        "Cumulative yield reported from the YieldManager contract",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LstLiabilityPrincipalGwei,
        "LST liability principal from the YieldManager contract",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastReportedNegativeYield,
        "Last reported negative yield from the YieldManager contract",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LidoLstLiabilityGwei,
        "Lido LST liability in gwei from Lido accounting reports",
        ["vault_address"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingPartialWithdrawalsGwei,
        "Total pending partial withdrawals in gwei",
        [],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalValidatorBalanceGwei,
        "Total validator balance in gwei",
        [],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingDepositGwei,
        "Total pending deposits in gwei",
        [],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingPartialWithdrawalQueueAmountGwei,
        "Pending partial withdrawal queue amount in gwei",
        ["pubkey", "withdrawable_epoch"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingDepositQueueAmountGwei,
        "Pending deposit queue amount in gwei",
        ["pubkey", "slot"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
        "Pending exit queue amount in gwei",
        ["pubkey", "exit_epoch", "slashed"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingExitGwei,
        "Total pending exit amount in gwei",
        [],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
        "Pending full withdrawal queue amount in gwei",
        ["pubkey", "withdrawable_epoch", "slashed"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingFullWithdrawalGwei,
        "Total pending full withdrawal amount in gwei",
        [],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ActualRebalanceRequirementGwei,
        "Original rebalance requirement (in gwei) before applying tolerance band, circuit breaker, or rate limit",
        ["vault_address", "staking_direction"],
      );
      expect(metricsService.createGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ReportedRebalanceRequirementGwei,
        "Reported rebalance requirement (in gwei) after applying tolerance band, circuit breaker, and rate limit",
        ["vault_address", "staking_direction"],
      );
    });

    it("registers histogram metrics with duration buckets", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();

      // Act
      new NativeYieldAutomationMetricsUpdater(metricsService);

      // Assert
      expect(metricsService.createHistogram).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
        [1, 5, 10, 30, 60, 120, 180, 300, 600, 900, 1200],
        "Operation mode execution duration in seconds",
        ["mode"],
      );
    });
  });

  describe("recordRebalance", () => {
    it("increments counter with positive amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.recordRebalance(RebalanceDirection.STAKE, 42);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.RebalanceAmountTotal,
        { direction: RebalanceDirection.STAKE },
        42,
      );
    });

    it("increments counter with zero amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.recordRebalance(RebalanceDirection.UNSTAKE, 0);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.RebalanceAmountTotal,
        { direction: RebalanceDirection.UNSTAKE },
        0,
      );
    });

    it("skips recording when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.recordRebalance(RebalanceDirection.UNSTAKE, -10);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });

    it("records rebalance with NONE direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.recordRebalance(RebalanceDirection.NONE, 0);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.RebalanceAmountTotal,
        { direction: RebalanceDirection.NONE },
        0,
      );
    });
  });

  describe("addValidatorPartialUnstakeAmount", () => {
    it("increments counter with positive amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addValidatorPartialUnstakeAmount(VALIDATOR_PUBKEY, 100);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorPartialUnstakeAmountTotal,
        { validator_pubkey: VALIDATOR_PUBKEY },
        100,
      );
    });

    it("skips recording when amount is zero", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addValidatorPartialUnstakeAmount(VALIDATOR_PUBKEY, 0);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });

    it("skips recording when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addValidatorPartialUnstakeAmount(VALIDATOR_PUBKEY, -1);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("incrementValidatorExit", () => {
    it("increments counter with default count of 1", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementValidatorExit(VALIDATOR_PUBKEY);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorExitTotal,
        { validator_pubkey: VALIDATOR_PUBKEY },
        1,
      );
    });

    it("increments counter with custom positive count", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementValidatorExit(VALIDATOR_PUBKEY, 5);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorExitTotal,
        { validator_pubkey: VALIDATOR_PUBKEY },
        5,
      );
    });

    it("skips recording when count is zero", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementValidatorExit(VALIDATOR_PUBKEY, 0);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });

    it("skips recording when count is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementValidatorExit(VALIDATOR_PUBKEY, -1);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("setValidatorStakedAmountGwei", () => {
    it("sets gauge with positive amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setValidatorStakedAmountGwei(VALIDATOR_PUBKEY, STANDARD_VALIDATOR_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorStakedAmountGwei,
        { pubkey: VALIDATOR_PUBKEY },
        STANDARD_VALIDATOR_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setValidatorStakedAmountGwei(VALIDATOR_PUBKEY, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorStakedAmountGwei,
        { pubkey: VALIDATOR_PUBKEY },
        0,
      );
    });

    it("skips setting gauge when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setValidatorStakedAmountGwei(VALIDATOR_PUBKEY, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("incrementLidoVaultAccountingReport", () => {
    it("increments counter for vault", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementLidoVaultAccountingReport(VAULT_ADDRESS);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LidoVaultAccountingReportSubmittedTotal,
        { vault_address: VAULT_ADDRESS },
      );
    });
  });

  describe("incrementReportYield", () => {
    it("increments counter for vault", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementReportYield(VAULT_ADDRESS);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal,
        { vault_address: VAULT_ADDRESS },
      );
    });
  });

  describe("setLastPeekedNegativeYieldReport", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastPeekedNegativeYieldReport(VAULT_ADDRESS, 123);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedNegativeYieldReport,
        { vault_address: VAULT_ADDRESS },
        123,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastPeekedNegativeYieldReport(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedNegativeYieldReport,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastPeekedNegativeYieldReport(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastPeekedPositiveYieldReport", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastPeekedPositiveYieldReport(VAULT_ADDRESS, 456);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedPositiveYieldReport,
        { vault_address: VAULT_ADDRESS },
        456,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastPeekedPositiveYieldReport(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedPositiveYieldReport,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastPeekedPositiveYieldReport(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastSettleableLidoFees", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastSettleableLidoFees(VAULT_ADDRESS, 789);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastSettleableLidoFees,
        { vault_address: VAULT_ADDRESS },
        789,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastSettleableLidoFees(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastSettleableLidoFees,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastSettleableLidoFees(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastVaultReportTimestamp", () => {
    it("sets gauge with positive timestamp", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastVaultReportTimestamp(VAULT_ADDRESS, SAMPLE_TIMESTAMP);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastVaultReportTimestamp,
        { vault_address: VAULT_ADDRESS },
        SAMPLE_TIMESTAMP,
      );
    });

    it("sets gauge with zero timestamp", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastVaultReportTimestamp(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastVaultReportTimestamp,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when timestamp is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastVaultReportTimestamp(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setYieldReportedCumulative", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setYieldReportedCumulative(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.YieldReportedCumulative,
        { vault_address: VAULT_ADDRESS },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setYieldReportedCumulative(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.YieldReportedCumulative,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setYieldReportedCumulative(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLstLiabilityPrincipalGwei", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLstLiabilityPrincipalGwei(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LstLiabilityPrincipalGwei,
        { vault_address: VAULT_ADDRESS },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLstLiabilityPrincipalGwei(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LstLiabilityPrincipalGwei,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLstLiabilityPrincipalGwei(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastReportedNegativeYield", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastReportedNegativeYield(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastReportedNegativeYield,
        { vault_address: VAULT_ADDRESS },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastReportedNegativeYield(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastReportedNegativeYield,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastReportedNegativeYield(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLidoLstLiabilityGwei", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLidoLstLiabilityGwei(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LidoLstLiabilityGwei,
        { vault_address: VAULT_ADDRESS },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLidoLstLiabilityGwei(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LidoLstLiabilityGwei,
        { vault_address: VAULT_ADDRESS },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLidoLstLiabilityGwei(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastTotalPendingPartialWithdrawalsGwei", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingPartialWithdrawalsGwei(SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingPartialWithdrawalsGwei,
        {},
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingPartialWithdrawalsGwei(0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingPartialWithdrawalsGwei,
        {},
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingPartialWithdrawalsGwei(-1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastTotalValidatorBalanceGwei", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalValidatorBalanceGwei(SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalValidatorBalanceGwei,
        {},
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalValidatorBalanceGwei(0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalValidatorBalanceGwei,
        {},
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalValidatorBalanceGwei(-1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastTotalPendingDepositGwei", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingDepositGwei(SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingDepositGwei,
        {},
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingDepositGwei(0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingDepositGwei,
        {},
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingDepositGwei(-1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setPendingPartialWithdrawalQueueAmountGwei", () => {
    it("sets gauge with positive amount and epoch", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingPartialWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, STANDARD_VALIDATOR_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingPartialWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: SAMPLE_EPOCH.toString() },
        STANDARD_VALIDATOR_AMOUNT_GWEI,
      );
    });

    it("converts epoch to string in labels", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingPartialWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, 12345, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingPartialWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: "12345" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingPartialWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingPartialWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: SAMPLE_EPOCH.toString() },
        0,
      );
    });

    it("sets gauge with zero epoch", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingPartialWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, 0, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingPartialWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: "0" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("skips setting gauge when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingPartialWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });

    it("skips setting gauge when epoch is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingPartialWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, -1, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setPendingDepositQueueAmountGwei", () => {
    it("sets gauge with positive amount and slot", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingDepositQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_SLOT, STANDARD_VALIDATOR_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingDepositQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, slot: SAMPLE_SLOT.toString() },
        STANDARD_VALIDATOR_AMOUNT_GWEI,
      );
    });

    it("converts slot to string in labels", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingDepositQueueAmountGwei(VALIDATOR_PUBKEY, 789012, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingDepositQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, slot: "789012" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingDepositQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_SLOT, 0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingDepositQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, slot: SAMPLE_SLOT.toString() },
        0,
      );
    });

    it("sets gauge with zero slot", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingDepositQueueAmountGwei(VALIDATOR_PUBKEY, 0, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingDepositQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, slot: "0" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("skips setting gauge when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingDepositQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_SLOT, -1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });

    it("skips setting gauge when slot is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingDepositQueueAmountGwei(VALIDATOR_PUBKEY, -1, SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setPendingExitQueueAmountGwei", () => {
    it("sets gauge with positive amount and epoch for non-slashed validator", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, STANDARD_VALIDATOR_AMOUNT_GWEI, false);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, exit_epoch: SAMPLE_EPOCH.toString(), slashed: "false" },
        STANDARD_VALIDATOR_AMOUNT_GWEI,
      );
    });

    it("sets gauge with slashed flag set to true", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, 12345, SAMPLE_AMOUNT_GWEI, true);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, exit_epoch: "12345", slashed: "true" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("converts epoch and slashed boolean to strings in labels", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, SAMPLE_AMOUNT_GWEI, true);
      jest.clearAllMocks();
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, SAMPLE_AMOUNT_GWEI, false);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, exit_epoch: SAMPLE_EPOCH.toString(), slashed: "false" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, 0, false);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, exit_epoch: SAMPLE_EPOCH.toString(), slashed: "false" },
        0,
      );
    });

    it("sets gauge with zero epoch", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, 0, SAMPLE_AMOUNT_GWEI, true);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingExitQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, exit_epoch: "0", slashed: "true" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("skips setting gauge when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, -1, false);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });

    it("skips setting gauge when epoch is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingExitQueueAmountGwei(VALIDATOR_PUBKEY, -1, SAMPLE_AMOUNT_GWEI, true);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastTotalPendingExitGwei", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingExitGwei(SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingExitGwei,
        {},
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingExitGwei(0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingExitGwei,
        {},
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingExitGwei(-1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setPendingFullWithdrawalQueueAmountGwei", () => {
    it("sets gauge with positive amount and epoch for non-slashed validator", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, STANDARD_VALIDATOR_AMOUNT_GWEI, false);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: SAMPLE_EPOCH.toString(), slashed: "false" },
        STANDARD_VALIDATOR_AMOUNT_GWEI,
      );
    });

    it("sets gauge with slashed flag set to true", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, 12345, SAMPLE_AMOUNT_GWEI, true);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: "12345", slashed: "true" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("converts epoch and slashed boolean to strings in labels", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, SAMPLE_AMOUNT_GWEI, true);
      jest.clearAllMocks();
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, SAMPLE_AMOUNT_GWEI, false);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: SAMPLE_EPOCH.toString(), slashed: "false" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, 0, false);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: SAMPLE_EPOCH.toString(), slashed: "false" },
        0,
      );
    });

    it("sets gauge with zero epoch", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, 0, SAMPLE_AMOUNT_GWEI, true);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.PendingFullWithdrawalQueueAmountGwei,
        { pubkey: VALIDATOR_PUBKEY, withdrawable_epoch: "0", slashed: "true" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("skips setting gauge when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, SAMPLE_EPOCH, -1, false);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });

    it("skips setting gauge when epoch is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setPendingFullWithdrawalQueueAmountGwei(VALIDATOR_PUBKEY, -1, SAMPLE_AMOUNT_GWEI, true);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastTotalPendingFullWithdrawalGwei", () => {
    it("sets gauge with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingFullWithdrawalGwei(SAMPLE_AMOUNT_GWEI);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingFullWithdrawalGwei,
        {},
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingFullWithdrawalGwei(0);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastTotalPendingFullWithdrawalGwei,
        {},
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setLastTotalPendingFullWithdrawalGwei(-1);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("addNodeOperatorFeesPaid", () => {
    it("increments counter with positive amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addNodeOperatorFeesPaid(VAULT_ADDRESS, 321);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.NodeOperatorFeesPaidTotal,
        { vault_address: VAULT_ADDRESS },
        321,
      );
    });

    it("skips recording when amount is zero", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addNodeOperatorFeesPaid(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });

    it("skips recording when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addNodeOperatorFeesPaid(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("addLiabilitiesPaid", () => {
    it("increments counter with positive amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addLiabilitiesPaid(VAULT_ADDRESS, 321);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LiabilitiesPaidTotal,
        { vault_address: VAULT_ADDRESS },
        321,
      );
    });

    it("skips recording when amount is zero", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addLiabilitiesPaid(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });

    it("skips recording when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addLiabilitiesPaid(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("addLidoFeesPaid", () => {
    it("increments counter with positive amount", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addLidoFeesPaid(VAULT_ADDRESS, 321);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LidoFeesPaidTotal,
        { vault_address: VAULT_ADDRESS },
        321,
      );
    });

    it("skips recording when amount is zero", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addLidoFeesPaid(VAULT_ADDRESS, 0);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });

    it("skips recording when amount is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.addLidoFeesPaid(VAULT_ADDRESS, -1);

      // Assert
      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("incrementOperationModeExecution", () => {
    it("increments counter with default success status", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementOperationModeExecution(OperationMode.OSSIFICATION_PENDING_MODE);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
        { mode: OperationMode.OSSIFICATION_PENDING_MODE, status: OperationModeExecutionStatus.Success },
      );
    });

    it("increments counter with explicit success status", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementOperationModeExecution(OperationMode.YIELD_REPORTING_MODE, OperationModeExecutionStatus.Success);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
        { mode: OperationMode.YIELD_REPORTING_MODE, status: OperationModeExecutionStatus.Success },
      );
    });

    it("increments counter with failure status", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementOperationModeExecution(OperationMode.YIELD_REPORTING_MODE, OperationModeExecutionStatus.Failure);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
        { mode: OperationMode.YIELD_REPORTING_MODE, status: OperationModeExecutionStatus.Failure },
      );
    });
  });

  describe("recordOperationModeDuration", () => {
    it("records duration with positive value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.recordOperationModeDuration(OperationMode.OSSIFICATION_COMPLETE_MODE, 120);

      // Assert
      expect(metricsService.addValueToHistogram).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
        120,
        { mode: OperationMode.OSSIFICATION_COMPLETE_MODE },
      );
    });

    it("records duration with zero value", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.recordOperationModeDuration(OperationMode.OSSIFICATION_COMPLETE_MODE, 0);

      // Assert
      expect(metricsService.addValueToHistogram).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
        0,
        { mode: OperationMode.OSSIFICATION_COMPLETE_MODE },
      );
    });

    it("skips recording when duration is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.recordOperationModeDuration(OperationMode.YIELD_REPORTING_MODE, -1);

      // Assert
      expect(metricsService.addValueToHistogram).not.toHaveBeenCalled();
    });
  });

  describe("incrementStakingDepositQuotaExceeded", () => {
    it("increments counter for vault", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementStakingDepositQuotaExceeded(VAULT_ADDRESS);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.StakingDepositQuotaExceeded,
        { vault_address: VAULT_ADDRESS },
      );
    });
  });

  describe("incrementContractEstimateGasError", () => {
    it("increments counter with contract address, revert data, and error name", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementContractEstimateGasError(CONTRACT_ADDRESS, RAW_REVERT_DATA, ERROR_NAME);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ContractEstimateGasError,
        {
          contract_address: CONTRACT_ADDRESS,
          rawRevertData: RAW_REVERT_DATA,
          errorName: ERROR_NAME,
        },
      );
    });

    it("defaults to unknown error name when not provided", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.incrementContractEstimateGasError(CONTRACT_ADDRESS, RAW_REVERT_DATA);

      // Assert
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ContractEstimateGasError,
        {
          contract_address: CONTRACT_ADDRESS,
          rawRevertData: RAW_REVERT_DATA,
          errorName: "unknown",
        },
      );
    });
  });

  describe("setActualRebalanceRequirement", () => {
    it("sets gauge with positive value and STAKE direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setActualRebalanceRequirement(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI, RebalanceDirection.STAKE);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ActualRebalanceRequirementGwei,
        { vault_address: VAULT_ADDRESS, staking_direction: "STAKING" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with positive value and UNSTAKE direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setActualRebalanceRequirement(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI, RebalanceDirection.UNSTAKE);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ActualRebalanceRequirementGwei,
        { vault_address: VAULT_ADDRESS, staking_direction: "UNSTAKING" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value and NONE direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setActualRebalanceRequirement(VAULT_ADDRESS, 0, RebalanceDirection.NONE);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ActualRebalanceRequirementGwei,
        { vault_address: VAULT_ADDRESS, staking_direction: "NONE" },
        0,
      );
    });

    it("defaults to NONE for unknown direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setActualRebalanceRequirement(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI, "INVALID" as RebalanceDirection);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ActualRebalanceRequirementGwei,
        { vault_address: VAULT_ADDRESS, staking_direction: "NONE" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setActualRebalanceRequirement(VAULT_ADDRESS, -1, RebalanceDirection.STAKE);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setReportedRebalanceRequirement", () => {
    it("sets gauge with positive value and STAKE direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setReportedRebalanceRequirement(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI, RebalanceDirection.STAKE);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ReportedRebalanceRequirementGwei,
        { vault_address: VAULT_ADDRESS, staking_direction: "STAKING" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with positive value and UNSTAKE direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setReportedRebalanceRequirement(VAULT_ADDRESS, SAMPLE_AMOUNT_GWEI, RebalanceDirection.UNSTAKE);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ReportedRebalanceRequirementGwei,
        { vault_address: VAULT_ADDRESS, staking_direction: "UNSTAKING" },
        SAMPLE_AMOUNT_GWEI,
      );
    });

    it("sets gauge with zero value and NONE direction", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setReportedRebalanceRequirement(VAULT_ADDRESS, 0, RebalanceDirection.NONE);

      // Assert
      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ReportedRebalanceRequirementGwei,
        { vault_address: VAULT_ADDRESS, staking_direction: "NONE" },
        0,
      );
    });

    it("skips setting gauge when value is negative", () => {
      // Arrange
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      // Act
      updater.setReportedRebalanceRequirement(VAULT_ADDRESS, -1, RebalanceDirection.STAKE);

      // Assert
      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });
});
