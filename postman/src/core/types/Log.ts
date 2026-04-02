import type { Address, Hash, Hex } from "./primitives";

export type Log = {
  address: Address;
  topics: Hex[];
  data: Hex;
  blockNumber: number;
  transactionHash: Hash;
  logIndex: number;
};
