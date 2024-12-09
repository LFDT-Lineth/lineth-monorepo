export type BlockRange = {
  startingBlock: number;
  endBlock: number;
};

export type FinalizationMessagingInfo = {
  l2MessagingBlocksRange: BlockRange;
  l2MerkleRoots: string[];
  treeDepth: number;
};

export type Proof = {
  proof: string[];
  root: string;
  leafIndex: number;
};

export interface IMerkleTreeService {
  getMessageProof(messageHash: string): Promise<Proof>;
  getFinalizationMessagingInfo(transactionHash: string): Promise<FinalizationMessagingInfo>;
  getL2MessageHashesInBlockRange(fromBlock: number, toBlock: number): Promise<string[]>;
  getMessageSiblings(messageHash: string, messageHashes: string[], treeDepth: number): string[];
}
