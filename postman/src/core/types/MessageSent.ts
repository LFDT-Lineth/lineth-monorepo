import type { Address, Hash, Hex } from "./primitives";

export type MessageSent = {
  messageHash: Hash;
  messageSender: Address;
  destination: Address;
  fee: bigint;
  value: bigint;
  messageNonce: bigint;
  calldata: Hex;
  contractAddress: Address;
  blockNumber: number;
  transactionHash: Hash;
  logIndex: number;
};
