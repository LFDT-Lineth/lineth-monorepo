import { Address, Hex } from "./misc";

export enum OnChainMessageStatus {
  UNKNOWN = "UNKNOWN",
  CLAIMABLE = "CLAIMABLE",
  CLAIMED = "CLAIMED",
}

export enum MessageDirection {
  L1_TO_L2 = "L1_TO_L2",
  L2_TO_L1 = "L2_TO_L1",
}

export type MessageProof = {
  proof: Hex[];
  root: Hex;
  leafIndex: number;
};

export type Message<T = bigint> = {
  from: Address;
  to: Address;
  fee: T;
  value: T;
  nonce: T;
  calldata: Hex;
  messageHash: Hex;
};

export type ExtendedMessage<T = bigint> = Message<T> & {
  transactionHash: Hex;
  blockNumber: T;
};
