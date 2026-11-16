import type { Address, Hex } from "viem";

export type BlobSubmission = {
  dataEvaluationClaim: Hex;
  kzgCommitment: Hex;
  kzgProof: Hex;
  finalStateRootHash: Hex;
  snarkHash: Hex;
};

export type ParentAndExpectedShnarf = {
  parentShnarf: Hex;
  expectedShnarf: Hex;
};

export type ShnarfData = {
  parentShnarf: Hex;
  snarkHash: Hex;
  finalStateRootHash: Hex;
  dataEvaluationPoint: Hex;
  dataEvaluationClaim: Hex;
};

export type CalldataSubmissionData = {
  finalStateRootHash: Hex;
  snarkHash: Hex;
  compressedData: Hex;
};

export type FinalizationData = {
  aggregatedProof: Hex;
  endBlockNumber: bigint;
  shnarfData: ShnarfData;
  parentStateRootHash: Hex;
  lastFinalizedTimestamp: bigint;
  finalTimestamp: bigint;
  l1RollingHash: Hex;
  l1RollingHashMessageNumber: bigint;
  l2MerkleRoots: Hex[];
  filteredAddresses: Address[];
  l2MerkleTreesDepth: bigint;
  l2MessagingBlocksOffsets: Hex;
  lastFinalizedL1RollingHash: Hex;
  lastFinalizedL1RollingHashMessageNumber: bigint;
  lastFinalizedForcedTransactionNumber: bigint;
  finalForcedTransactionNumber: bigint;
  lastFinalizedForcedTransactionRollingHash: Hex;
};

export type ShnarfDataGenerator = (blobParentShnarfIndex: number, isMultiple?: boolean) => ShnarfData;

export type Eip1559Transaction = {
  nonce: bigint;
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
  gasLimit: bigint;
  to: Address;
  value: bigint;
  input: Hex;
  accessList: AccessList[];
  yParity: number;
  r: bigint;
  s: bigint;
};

export type AccessList = {
  contractAddress: Address;
  storageKeys: Hex[];
};

export type AccessListEntryInput = {
  address: string;
  storageKeys: string[];
};

export type LastFinalizedState = {
  timestamp: bigint;
  messageNumber: bigint;
  messageRollingHash: Hex;
  forcedTransactionNumber: bigint;
  forcedTransactionRollingHash: Hex;
};

export type RoleAddress = {
  addressWithRole: Address;
  role: Hex;
};

export type PauseTypeRole = {
  pauseType: number;
  role: Hex;
};

export type LinethRollupInitializationData = {
  initialStateRootHash: Hex;
  initialL2BlockNumber: bigint;
  genesisTimestamp: bigint;
  defaultVerifier: Address;
  rateLimitPeriodInSeconds: bigint;
  rateLimitAmountInWei: bigint;
  roleAddresses: RoleAddress[];
  pauseTypeRoles: PauseTypeRole[];
  unpauseTypeRoles: PauseTypeRole[];
  defaultAdmin: Address;
  shnarfProvider: Address;
  addressFilter: Address;
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
