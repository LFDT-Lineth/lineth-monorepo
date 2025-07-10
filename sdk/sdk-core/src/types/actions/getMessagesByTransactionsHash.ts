import { ExtendedMessage } from "../message";
import { Hex } from "../misc";

export type GetMessagesByTransactionHashParameters = {
  transactionHash: Hex;
};

export type GetMessagesByTransactionHashReturnType<T = bigint> = ExtendedMessage<T>[];
