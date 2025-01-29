export type RawBlockData = {
  rootHash: string;
  timestamp: number;
  rlpEncodedTransactions: string[];
  l2ToL1MsgHashes: string[];
  batchReceptionIndices: number[];
  fromAddresses: string;
};

export type FormattedBlockData = Omit<
  RawBlockData,
  "rlpEncodedTransactions" | "timestamp" | "l2ToL1MsgHashes" | "fromAddresses" | "rootHash"
> & {
  l2BlockTimestamp: number;
  transactions: string[];
  l2ToL1MsgHashes: string[];
  fromAddresses: string;
  blockRootHash: string;
};

export type DebugData = {
  blocks: {
    txHashes: string[];
    hashOfTxHashes: string;
    logHashes: string[];
    hashOfLogHashes: string;
    hashOfPositions: string;
    HashForBlock: string;
  }[];
  hashForAllBlocks: string;
  hashOfRootHashes: string;
  timestampHashes: string;
  finalHash: string;
};

export type BlobSubmission = {
  dataEvaluationClaim: string;
  kzgCommitment: string;
  kzgProof: string;
  finalStateRootHash: string;
  snarkHash: string;
};

export type ParentSubmissionData = {
  finalStateRootHash: string;
  firstBlockNumber: bigint;
  endBlockNumber: bigint;
  shnarf: string;
};

export type ParentAndExpectedShnarf = {
  parentShnarf: string;
  expectedShnarf: string;
};

export type ShnarfData = {
  parentShnarf: string;
  snarkHash: string;
  finalStateRootHash: string;
  dataEvaluationPoint: string;
  dataEvaluationClaim: string;
};

export type CalldataSubmissionData = {
  finalStateRootHash: string;
  snarkHash: string;
  compressedData: string;
};

export type FinalizationData = {
  aggregatedProof: string;
  endBlockNumber: bigint;
  shnarfData: ShnarfData;
  parentStateRootHash: string;
  lastFinalizedTimestamp: bigint;
  finalTimestamp: bigint;
  l1RollingHash: string;
  l1RollingHashMessageNumber: bigint;
  l2MerkleRoots: string[];
  l2MerkleTreesDepth: bigint;
  l2MessagingBlocksOffsets: string;
  lastFinalizedL1RollingHash: string;
  lastFinalizedL1RollingHashMessageNumber: bigint;
};

export type ShnarfDataGenerator = (blobParentShnarfIndex: number, isMultiple?: boolean) => ShnarfData;
