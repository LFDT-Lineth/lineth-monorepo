export type BlobSubmission = {
  /** Final L2 block hash for this blob (submitBlobs argument). */
  finalBlockHash: string;
  /** EIP-4844 versioned blob hash (blobhash(i)); used when computing expected shnarf. */
  dataHash: string;
  compressedData: string;
  /** Kept for fixture/debug; not sent on-chain. */
  kzgCommitment?: string;
};

export type ParentAndExpectedShnarf = {
  parentShnarf: string;
  expectedShnarf: string;
};

/** Legacy 5-field shnarf layout retained as FinalizationDataV5 ABI padding. */
export type ShnarfData = {
  parentShnarf: string;
  snarkHash: string;
  finalStateRootHash: string;
  dataEvaluationPoint: string;
  dataEvaluationClaim: string;
};

export type CalldataSubmissionData = {
  blockHash: string;
  compressedData: string;
};

export type FinalizationData = {
  aggregatedProof: string;
  endBlockNumber: bigint;
  shnarfData: ShnarfData;
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
  finalBlobHash: string;
  verifierKeys: string[];
};

export type ShnarfDataGenerator = (blobParentShnarfIndex: number, isMultiple?: boolean) => ShnarfData;

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

export type LineaRollupInitializationData = {
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
