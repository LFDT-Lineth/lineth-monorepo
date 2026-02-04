import type { ILogger } from "@consensys/linea-shared-utils";
import type { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";

/**
 * Creates a mock logger for testing.
 */
export const createLoggerMock = (): jest.Mocked<ILogger> => ({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

/**
 * Creates a mock metrics updater with all methods stubbed.
 */
export const createMetricsUpdaterMock = (): jest.Mocked<INativeYieldAutomationMetricsUpdater> => ({
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
});
