import { Hash, TransactionReceipt } from "../types";

export interface IReceiptPoller {
  poll(transactionHash: Hash, timeout: number, interval: number): Promise<TransactionReceipt>;
}
