import { jest } from "@jest/globals";
import { IMetricsService } from "@consensys/linea-shared-utils";
import { NativeYieldAutomationMetricsUpdater } from "../NativeYieldAutomationMetricsUpdater.js";
import {
  LineaNativeYieldAutomationServiceMetrics,
  OperationTrigger,
} from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import { OperationMode } from "../../../core/enums/OperationModeEnums.js";
import { Address, Hex } from "viem";

const createMetricsServiceMock = (): jest.Mocked<IMetricsService<LineaNativeYieldAutomationServiceMetrics>> =>
  ({
    getRegistry: jest.fn(),
    createCounter: jest.fn(),
    createGauge: jest.fn(),
    incrementCounter: jest.fn(),
    setGauge: jest.fn(),
    incrementGauge: jest.fn(),
    decrementGauge: jest.fn(),
    getGaugeValue: jest.fn(),
    getCounterValue: jest.fn(),
    createHistogram: jest.fn(),
    addValueToHistogram: jest.fn(),
    getHistogramMetricsValues: jest.fn(),
  }) as unknown as jest.Mocked<IMetricsService<LineaNativeYieldAutomationServiceMetrics>>;

describe("NativeYieldAutomationMetricsUpdater", () => {
  const validatorPubkey = "0xvalidator" as Hex;
  const vaultAddress = "0xvault" as Address;

  it("registers all metrics on construction", () => {
    const metricsService = createMetricsServiceMock();

    // Constructing should immediately register all metrics
    new NativeYieldAutomationMetricsUpdater(metricsService);

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
      LineaNativeYieldAutomationServiceMetrics.ReportYieldAmountTotal,
      "Total yield amount reported per vault",
      ["vault_address"],
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
      LineaNativeYieldAutomationServiceMetrics.LastPeekUnpaidLidoProtocolFees,
      "Unpaid Lido protocol fees from the last peek",
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
      LineaNativeYieldAutomationServiceMetrics.OperationModeTriggerTotal,
      "Operation mode triggers grouped by mode and triggers",
      ["mode", "trigger"],
    );
    expect(metricsService.createCounter).toHaveBeenCalledWith(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
      "Operation mode executions grouped by mode",
      ["mode"],
    );
    expect(metricsService.createHistogram).toHaveBeenCalledWith(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
      [1, 5, 10, 30, 60, 120, 180, 300, 600, 900, 1200],
      "Operation mode execution duration in seconds",
      ["mode"],
    );
  });

  describe("recordRebalance", () => {
    it("increments counter when amount is positive", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.recordRebalance(RebalanceDirection.STAKE, 42);

      expect(metricsService.incrementCounter).toHaveBeenCalledTimes(1);
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.RebalanceAmountTotal,
        { direction: RebalanceDirection.STAKE },
        42,
      );
    });

    it("does not increment when amount is non-positive", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.recordRebalance(RebalanceDirection.UNSTAKE, 0);
      updater.recordRebalance(RebalanceDirection.UNSTAKE, -10);

      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("addValidatorPartialUnstakeAmount", () => {
    it("increments counter when amount is positive", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.addValidatorPartialUnstakeAmount(validatorPubkey, 100);

      expect(metricsService.incrementCounter).toHaveBeenCalledTimes(1);
      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorPartialUnstakeAmountTotal,
        { validator_pubkey: validatorPubkey },
        100,
      );
    });

    it("does not increment when amount is non-positive", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.addValidatorPartialUnstakeAmount(validatorPubkey, 0);

      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("incrementValidatorExit", () => {
    it("defaults count to 1", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.incrementValidatorExit(validatorPubkey);

      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ValidatorExitTotal,
        { validator_pubkey: validatorPubkey },
        1,
      );
    });

    it("does not increment when count is non-positive", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.incrementValidatorExit(validatorPubkey, 0);

      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  it("increments accounting and report counters", () => {
    const metricsService = createMetricsServiceMock();
    const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
    jest.clearAllMocks();

    updater.incrementLidoVaultAccountingReport(vaultAddress);
    updater.incrementReportYield(vaultAddress);

    expect(metricsService.incrementCounter).toHaveBeenCalledWith(
      LineaNativeYieldAutomationServiceMetrics.LidoVaultAccountingReportSubmittedTotal,
      { vault_address: vaultAddress },
    );
    expect(metricsService.incrementCounter).toHaveBeenCalledWith(
      LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal,
      { vault_address: vaultAddress },
    );
  });

  describe("addReportedYieldAmount", () => {
    it("increments counter for positive amount", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.addReportedYieldAmount(vaultAddress, 500);

      expect(metricsService.incrementCounter).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.ReportYieldAmountTotal,
        { vault_address: vaultAddress },
        500,
      );
    });

    it("does not increment for non-positive amount", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.addReportedYieldAmount(vaultAddress, 0);

      expect(metricsService.incrementCounter).not.toHaveBeenCalled();
    });
  });

  describe("setLastPeekedNegativeYieldReport", () => {
    it("sets gauge when value is non-negative", async () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      await updater.setLastPeekedNegativeYieldReport(vaultAddress, 123);

      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedNegativeYieldReport,
        { vault_address: vaultAddress },
        123,
      );
    });

    it("does not set gauge when value is negative", async () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      await updater.setLastPeekedNegativeYieldReport(vaultAddress, -1);

      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastPeekedPositiveYieldReport", () => {
    it("sets gauge when value is non-negative", async () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      await updater.setLastPeekedPositiveYieldReport(vaultAddress, 456);

      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekedPositiveYieldReport,
        { vault_address: vaultAddress },
        456,
      );
    });

    it("does not set gauge when value is negative", async () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      await updater.setLastPeekedPositiveYieldReport(vaultAddress, -1);

      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("setLastPeekUnpaidLidoProtocolFees", () => {
    it("sets gauge when value is non-negative", async () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      await updater.setLastPeekUnpaidLidoProtocolFees(vaultAddress, 789);

      expect(metricsService.setGauge).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.LastPeekUnpaidLidoProtocolFees,
        { vault_address: vaultAddress },
        789,
      );
    });

    it("does not set gauge when value is negative", async () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      await updater.setLastPeekUnpaidLidoProtocolFees(vaultAddress, -1);

      expect(metricsService.setGauge).not.toHaveBeenCalled();
    });
  });

  describe("vault payout counters", () => {
    const cases: Array<{
      metric: LineaNativeYieldAutomationServiceMetrics;
      invoke: (updater: NativeYieldAutomationMetricsUpdater, address: Address, amount: number) => void;
    }> = [
      {
        metric: LineaNativeYieldAutomationServiceMetrics.NodeOperatorFeesPaidTotal,
        invoke: (updater, address, amount) => updater.addNodeOperatorFeesPaid(address, amount),
      },
      {
        metric: LineaNativeYieldAutomationServiceMetrics.LiabilitiesPaidTotal,
        invoke: (updater, address, amount) => updater.addLiabilitiesPaid(address, amount),
      },
      {
        metric: LineaNativeYieldAutomationServiceMetrics.LidoFeesPaidTotal,
        invoke: (updater, address, amount) => updater.addLidoFeesPaid(address, amount),
      },
    ];

    cases.forEach(({ metric, invoke }) => {
      it(`increments ${metric} when amount is positive`, () => {
        const metricsService = createMetricsServiceMock();
        const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
        jest.clearAllMocks();

        invoke(updater, vaultAddress, 321);

        expect(metricsService.incrementCounter).toHaveBeenCalledWith(metric, { vault_address: vaultAddress }, 321);
      });

      it(`does not increment ${metric} when amount is non-positive`, () => {
        const metricsService = createMetricsServiceMock();
        const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
        jest.clearAllMocks();

        invoke(updater, vaultAddress, 0);

        expect(metricsService.incrementCounter).not.toHaveBeenCalled();
      });
    });
  });

  it("increments operation mode trigger counter", () => {
    const metricsService = createMetricsServiceMock();
    const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
    jest.clearAllMocks();

    updater.incrementOperationModeTrigger(OperationMode.YIELD_REPORTING_MODE, OperationTrigger.TIMEOUT);

    expect(metricsService.incrementCounter).toHaveBeenCalledWith(
      LineaNativeYieldAutomationServiceMetrics.OperationModeTriggerTotal,
      { mode: OperationMode.YIELD_REPORTING_MODE, trigger: OperationTrigger.TIMEOUT },
    );
  });

  it("increments operation mode execution counter", () => {
    const metricsService = createMetricsServiceMock();
    const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
    jest.clearAllMocks();

    updater.incrementOperationModeExecution(OperationMode.OSSIFICATION_PENDING_MODE);

    expect(metricsService.incrementCounter).toHaveBeenCalledWith(
      LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionTotal,
      { mode: OperationMode.OSSIFICATION_PENDING_MODE },
    );
  });

  describe("recordOperationModeDuration", () => {
    it("records duration when value is non-negative", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.recordOperationModeDuration(OperationMode.OSSIFICATION_COMPLETE_MODE, 0);

      expect(metricsService.addValueToHistogram).toHaveBeenCalledWith(
        LineaNativeYieldAutomationServiceMetrics.OperationModeExecutionDurationSeconds,
        0,
        { mode: OperationMode.OSSIFICATION_COMPLETE_MODE },
      );
    });

    it("does not record when duration is negative", () => {
      const metricsService = createMetricsServiceMock();
      const updater = new NativeYieldAutomationMetricsUpdater(metricsService);
      jest.clearAllMocks();

      updater.recordOperationModeDuration(OperationMode.YIELD_REPORTING_MODE, -1);

      expect(metricsService.addValueToHistogram).not.toHaveBeenCalled();
    });
  });
});
