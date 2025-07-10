import { Message, MessageProof } from "../message";
import { Address, Hex } from "../misc";
import { TransactionRequest } from "../transaction";

export type ClaimOnL1Parameters<TUnit = bigint, TType = string> = Omit<
  TransactionRequest<TUnit, TType>,
  "from" | "data" | "to"
> &
  Omit<Message, "messageHash"> & { messageProof: MessageProof; feeRecipient?: Address };

export type ClaimOnL1ReturnType = Hex;
