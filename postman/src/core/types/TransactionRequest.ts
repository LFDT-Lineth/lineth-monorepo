import type { Address, Hex } from "./primitives";

export type TransactionRequest = {
  from?: Address;
  to: Address;
  data?: Hex;
  value?: bigint;
  gasLimit?: bigint;
  maxFeePerGas?: bigint;
  maxPriorityFeePerGas?: bigint;
};
