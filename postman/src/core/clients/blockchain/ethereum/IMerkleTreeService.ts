import type { Hash, Hex } from "../../../types/primitives";

export type BlockRange = {
  startingBlock: number;
  endBlock: number;
};

export type FinalizationMessagingInfo = {
  l2MessagingBlocksRange: BlockRange;
  l2MerkleRoots: Hash[];
  treeDepth: number;
};

export type Proof = {
  proof: Hex[];
  root: Hash;
  leafIndex: number;
};

export interface IMerkleTreeService {
  getMessageProof(messageHash: Hash): Promise<Proof>;
  getFinalizationMessagingInfo(transactionHash: Hash): Promise<FinalizationMessagingInfo>;
  getL2MessageHashesInBlockRange(fromBlock: number, toBlock: number): Promise<Hash[]>;
  getMessageSiblings(messageHash: Hash, messageHashes: Hash[], treeDepth: number): Hash[];
}
