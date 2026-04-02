import { Log } from "./Log";

import type { Hash } from "./primitives";

export type TransactionReceipt = {
  hash: Hash;
  blockNumber: number;
  status: "success" | "reverted";
  gasUsed: bigint;
  gasPrice: bigint;
  logs: Log[];
};
