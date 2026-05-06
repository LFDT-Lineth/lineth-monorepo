import { Address, Hex } from "../misc";
import { TransactionRequest } from "../transaction";

export type WithdrawParameters<TUnit = bigint, TType = string> = Omit<
  TransactionRequest<TUnit, TType>,
  "from" | "data" | "to"
> & {
  token: Address;
  to: Address;
  amount: bigint;
  data?: Hex;
};

export type WithdrawReturnType = Hex;
