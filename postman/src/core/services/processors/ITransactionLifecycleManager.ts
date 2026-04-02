import { Message } from "../../entities/Message";
import { TransactionReceipt } from "../../types";

export interface ITransactionLifecycleManager {
  retryWithBump(message: Message): Promise<TransactionReceipt | null>;
  cancelAndResetMessage(message: Message): Promise<TransactionReceipt | null>;
}

export type TransactionLifecycleConfig = {
  receiptPollingTimeout: number;
  receiptPollingInterval: number;
};
