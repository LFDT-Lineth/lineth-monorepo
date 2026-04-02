import type { Hash } from "./primitives";

export type TransactionSubmission = {
  hash: Hash;
  nonce: number;
  gasLimit: bigint;
  maxFeePerGas?: bigint;
  maxPriorityFeePerGas?: bigint;
};
