import { Address, TransactionReceipt } from "viem";
import { Result } from "neverthrow";

export interface IOperationModeMetricsRecorder {
  recordProgressOssificationMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void>;

  recordReportYieldMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void>;

  recordSafeWithdrawalMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void>;

  recordTransferFundsMetrics(
    yieldProvider: Address,
    txReceiptResult: Result<TransactionReceipt | undefined, Error>,
  ): Promise<void>;
}
