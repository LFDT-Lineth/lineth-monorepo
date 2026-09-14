export type BlobSubmission = {
  /** EIP-4844 versioned blob hash (blobhash(i)); folded into the dataRollingHash on-chain. */
  dataHash: string;
  compressedData: string;
  /** Kept for fixture/debug; not sent on-chain. */
  kzgCommitment?: string;
};

/** A single chunk in the DA stream, tracking the running dataRollingHash accumulator. */
export type DataRollingChainEntry = {
  compressedData: string;
  /** Chunk hash folded into the accumulator (keccak256(compressedData) for calldata, blobhash(i) for blobs). */
  chunkHash: string;
  /** Accumulator value before folding this chunk. */
  parentDataRollingHash: string;
  /** Accumulator value after folding this chunk. */
  dataRollingHash: string;
  /** Byte length of compressedData (used to derive stream offsets). */
  byteLength: number;
};

export type ParentAndExpectedDataRollingHash = {
  parentDataRollingHash: string;
  expectedDataRollingHash: string;
};

export type CalldataSubmissionData = {
  compressedData: string;
};

export type FinalizationData = {
  aggregatedProof: string;
  endBlockNumber: bigint;
  parentStateRootHash: string;
  parentBlockHash: string;
  lastFinalizedTimestamp: bigint;
  finalTimestamp: bigint;
  l1RollingHash: string;
  l1RollingHashMessageNumber: bigint;
  l2MerkleRoots: string[];
  filteredAddresses: string[];
  l2MerkleTreesDepth: bigint;
  l2MessagingBlocksOffsets: string;
  lastFinalizedL1RollingHash: string;
  lastFinalizedL1RollingHashMessageNumber: bigint;
  lastFinalizedForcedTransactionNumber: bigint;
  finalForcedTransactionNumber: bigint;
  lastFinalizedForcedTransactionRollingHash: string;
  finalBlockHash: string;
  prevDataRollingHash: string;
  prevOffset: bigint;
  parentDataRollingHash: string;
  endDataRollingHash: string;
  startOffset: bigint;
  endOffset: bigint;
  verifierKeys: string[];
};

export type ShnarfDataGenerator = (
  blobParentShnarfIndex: number,
  isMultiple?: boolean,
) => ParentAndExpectedDataRollingHash;

export type Eip1559Transaction = {
  nonce: bigint;
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
  gasLimit: bigint;
  to: string;
  value: bigint;
  input: string;
  accessList: AccessList[];
  yParity: bigint;
  r: bigint;
  s: bigint;
};

export type AccessList = {
  contractAddress: string;
  storageKeys: string[];
};

export type AccessListEntryInput = {
  address: string;
  storageKeys: string[];
};

export type LastFinalizedState = {
  timestamp: bigint;
  messageNumber: bigint;
  messageRollingHash: string;
  forcedTransactionNumber: bigint;
  forcedTransactionRollingHash: string;
};

export type RoleAddress = {
  addressWithRole: string;
  role: string;
};

export type PauseTypeRole = {
  pauseType: string;
  role: string;
};

export type LinethRollupInitializationData = {
  initialBlockHash: string;
  initialL2BlockNumber: bigint;
  genesisTimestamp: bigint;
  defaultVerifier: string;
  rateLimitPeriodInSeconds: bigint;
  rateLimitAmountInWei: bigint;
  roleAddresses: RoleAddress[];
  pauseTypeRoles: PauseTypeRole[];
  unpauseTypeRoles: PauseTypeRole[];
  verifierKeys: string[];
  defaultAdmin: string;
  shnarfProvider: string;
  addressFilter: string;
};

export type AggregatedProofData = {
  finalShnarf: string;
  parentAggregationFinalShnarf: string;
  aggregatedProof: string;
  aggregatedProverVersion: string;
  aggregatedVerifierIndex: number;
  aggregatedProofPublicInput: string;
  dataHashes: string[];
  dataParentHash: string;
  finalStateRootHash: string;
  parentStateRootHash: string;
  parentAggregationLastBlockTimestamp: number;
  lastFinalizedBlockNumber: number;
  finalTimestamp: number;
  finalBlockNumber: number;
  lastFinalizedL1RollingHash: string;
  l1RollingHash: string;
  lastFinalizedL1RollingHashMessageNumber: number;
  l1RollingHashMessageNumber: number;
  finalFtxRollingHash: string;
  parentAggregationFtxRollingHash: string;
  finalFtxNumber: number;
  parentAggregationFtxNumber: number;
  l2MerkleRoots: string[];
  l2MerkleTreesDepth: number;
  l2MessagingBlocksOffsets: string;
  chainID: number;
  baseFee: number;
  coinBase: string;
  l2MessageServiceAddr: string;
  isAllowedCircuitID: number;
  filteredAddresses: string[];
};

export type ExpectedCustomError = {
  name: string;
  args?: unknown[];
};
