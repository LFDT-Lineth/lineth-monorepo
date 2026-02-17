import { Address, Hash, Hex } from "viem";

export type MessageEvent = {
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  messageNumber: bigint;
  calldata: Hex;
  messageHash: Hash;
  blockNumber: bigint;
};
