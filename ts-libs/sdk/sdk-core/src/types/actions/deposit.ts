import { Address, Hex } from "../misc";
import { TransactionRequest } from "../transaction";

export type DepositParameters<TClient, TUnit = bigint, TType = string> = Omit<
  TransactionRequest<TUnit, TType>,
  "from" | "data" | "to"
> & {
  l2Client: TClient;
  token: Address;
  to: Address;
  fee?: TUnit;
  amount: TUnit;
  data?: Hex;
};

export type DepositReturnType = Hex;
