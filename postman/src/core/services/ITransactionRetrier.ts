import { Hash, TransactionSubmission } from "../types";

export interface ITransactionRetrier {
  retryWithHigherFee(transactionHash: Hash, attempt: number): Promise<TransactionSubmission>;
  cancelTransaction(nonce: number, stuckFees?: { maxFeePerGas: bigint; maxPriorityFeePerGas: bigint }): Promise<Hash>;
}
