import { Address, Hash, Hex } from "viem";

export type FinalizationData = {
  aggregatedProof: string;
  aggregatedProverVersion: string;
  aggregatedVerifierIndex: number;
  aggregatedProofPublicInput: string;
  dataHashes: string[];
  dataParentHash: string;
  parentStateRootHash: string;
  parentAggregationLastBlockTimestamp: number;
  finalTimestamp: number;
  finalBlockNumber: number;
  l1RollingHash: string;
  l1RollingHashMessageNumber: number;
  l2MerkleRoots: string[];
  l2MerkleTreesDepth: number;
  l2MessagingBlocksOffsets: string;
};

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

export type SendMessageArgs = {
  to: Address;
  fee: bigint;
  calldata: Hex;
};
