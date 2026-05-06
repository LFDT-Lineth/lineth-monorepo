import { Hex } from "../misc";
import { TransactionReceipt } from "../transaction";

export type GetTransactionReceiptByMessageHashParameters = {
  messageHash: Hex;
};

export type GetTransactionReceiptByMessageHashReturnType<T = bigint> = TransactionReceipt<T>;
