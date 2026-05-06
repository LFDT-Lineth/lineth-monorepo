import { Message } from "../message";
import { Address, Hex } from "../misc";
import { TransactionRequest } from "../transaction";

export type ClaimOnL2Parameters<TUnit = bigint, TType = string> = Omit<
  TransactionRequest<TUnit, TType>,
  "from" | "data" | "to"
> &
  Omit<Message, "messageHash"> & { feeRecipient?: Address };

export type ClaimOnL2ReturnType = Hex;
