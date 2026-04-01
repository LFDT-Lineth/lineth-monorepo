import { Result } from "neverthrow";
import { Address, TransactionReceipt } from "viem";

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
