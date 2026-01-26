/**
 * Contract Integrity Verifier - TypeScript Types
 *
 * Types for the bytecode verification tool that compares deployed contracts
 * against local artifact files.
 */

export interface ChainConfig {
  chainId: number;
  rpcUrl: string;
  explorerUrl?: string;
}

export interface ContractConfig {
  name: string;
  chain: string;
  address: string;
  artifactFile: string;
  isProxy?: boolean;
  /**
   * Constructor arguments for contracts with immutables.
   * Can be provided as:
   * - Array of values (will be ABI-encoded using constructor ABI)
   * - Hex string of already-encoded args
   */
  constructorArgs?: unknown[] | string;
  /**
   * Optional state verification configuration.
   * Used to verify contract state after initialization/upgrade.
   */
  stateVerification?: StateVerificationConfig;
}

// ============================================================================
// State Verification Types
// ============================================================================

export interface StateVerificationConfig {
  /** OpenZeppelin version for storage layout patterns. Default: "auto" */
  ozVersion?: "v4" | "v5" | "auto";
  /** Path to storage schema file (for path-based verification) */
  schemaFile?: string;
  /** View function calls to verify */
  viewCalls?: ViewCallConfig[];
  /** ERC-7201 namespaced storage verification */
  namespaces?: NamespaceConfig[];
  /** Explicit storage slot verification */
  slots?: SlotConfig[];
  /** Storage path-based verification (requires schemaFile) */
  storagePaths?: StoragePathConfig[];
}

export interface ViewCallConfig {
  /** Function name (will lookup signature in ABI) */
  function: string;
  /** Optional parameters for the function call */
  params?: unknown[];
  /** Expected return value */
  expected: unknown;
  /** Comparison mode. Default: "eq" */
  comparison?: "eq" | "gt" | "gte" | "lt" | "lte" | "contains";
}

export interface NamespaceConfig {
  /** ERC-7201 namespace identifier (e.g., "linea.storage.YieldManager") */
  id: string;
  /** Variables within the namespace to verify */
  variables: NamespaceVariable[];
}

export interface NamespaceVariable {
  /** Slot offset within the namespace (0, 1, 2, ...) */
  offset: number;
  /** Solidity type for decoding */
  type: SlotType;
  /** Variable name for display */
  name: string;
  /** Expected value */
  expected: unknown;
}

export type SlotType =
  | "address"
  | "uint256"
  | "uint128"
  | "uint96"
  | "uint64"
  | "uint32"
  | "uint16"
  | "uint8"
  | "int256"
  | "int128"
  | "int96"
  | "int64"
  | "int32"
  | "int16"
  | "int8"
  | "bool"
  | "bytes32";

export interface SlotConfig {
  /** Storage slot (hex string, e.g., "0x0") */
  slot: string;
  /** Solidity type for decoding */
  type: SlotType;
  /** Variable name for display */
  name: string;
  /** Expected value */
  expected: unknown;
  /** Byte offset within the slot (for packed storage). Default: 0 */
  offset?: number;
}

// ============================================================================
// Storage Path Types (ERC-7201 Schema-based)
// ============================================================================

export type SolidityType =
  | "address"
  | "bool"
  | "uint8"
  | "uint16"
  | "uint32"
  | "uint64"
  | "uint96"
  | "uint128"
  | "uint256"
  | "int8"
  | "int16"
  | "int32"
  | "int64"
  | "int96"
  | "int128"
  | "int256"
  | "bytes32"
  | "bytes4"
  | `address[]`
  | `uint256[]`
  | `mapping(${string} => ${string})`;

export interface StorageFieldDef {
  /** Slot offset from struct base */
  slot: number;
  /** Solidity type */
  type: SolidityType;
  /** Byte offset within slot (for packed storage) */
  byteOffset?: number;
}

export interface StorageStructDef {
  /** ERC-7201 namespace ID (e.g., "linea.storage.YieldManagerStorage") */
  namespace?: string;
  /** Pre-computed base slot (optional, will calculate from namespace if not provided) */
  baseSlot?: string;
  /** Field definitions */
  fields: Record<string, StorageFieldDef>;
}

export interface StorageSchema {
  /** Schema definitions by struct name */
  structs: Record<string, StorageStructDef>;
}

export interface StoragePathConfig {
  /** Storage path (e.g., "YieldManagerStorage:yieldManager" or "LineaRollupStorage:yieldManager") */
  path: string;
  /** Expected value */
  expected: unknown;
  /** Comparison mode. Default: "eq" */
  comparison?: "eq" | "gt" | "gte" | "lt" | "lte";
}

export interface StoragePathResult {
  path: string;
  computedSlot: string;
  type: string;
  expected: unknown;
  actual: unknown;
  status: VerificationStatus;
  message: string;
}

export interface ViewCallResult {
  function: string;
  params: unknown[] | undefined;
  expected: unknown;
  actual: unknown;
  status: VerificationStatus;
  message: string;
}

export interface SlotResult {
  slot: string;
  name: string;
  expected: unknown;
  actual: unknown;
  status: VerificationStatus;
  message: string;
}

export interface NamespaceResult {
  namespaceId: string;
  baseSlot: string;
  variables: SlotResult[];
  status: VerificationStatus;
}

export interface StateVerificationResult {
  status: VerificationStatus;
  message: string;
  viewCallResults: ViewCallResult[] | undefined;
  namespaceResults: NamespaceResult[] | undefined;
  slotResults: SlotResult[] | undefined;
  storagePathResults: StoragePathResult[] | undefined;
}

export interface ImmutableReference {
  start: number; // byte position in deployed bytecode
  length: number; // always 32 for immutables
}

export interface ImmutableDifference {
  position: number;
  length: number;
  localValue: string;
  remoteValue: string;
  possibleType: string | undefined;
}

export interface VerifierConfig {
  chains: Record<string, ChainConfig>;
  contracts: ContractConfig[];
}

// ============================================================================
// Artifact Types (Hardhat & Foundry)
// ============================================================================

export type ArtifactFormat = "hardhat" | "foundry";

/**
 * Hardhat artifact format (hh-sol-artifact-1)
 */
export interface HardhatArtifact {
  _format?: string;
  contractName: string;
  sourceName?: string;
  abi: AbiElement[];
  bytecode: string;
  deployedBytecode: string;
  linkReferences?: Record<string, unknown>;
  deployedLinkReferences?: Record<string, unknown>;
}

/**
 * Foundry artifact format (forge build output)
 */
export interface FoundryArtifact {
  abi: AbiElement[];
  bytecode: {
    object: string;
    sourceMap?: string;
    linkReferences?: Record<string, unknown>;
  };
  deployedBytecode: {
    object: string;
    sourceMap?: string;
    linkReferences?: Record<string, unknown>;
    immutableReferences?: Record<string, FoundryImmutableRef[]>;
  };
  methodIdentifiers?: Record<string, string>;
  rawMetadata?: string;
  metadata?: unknown;
  ast?: unknown;
  id?: number;
}

export interface FoundryImmutableRef {
  start: number;
  length: number;
}

/**
 * Normalized artifact format used internally.
 * Supports both Hardhat and Foundry artifacts.
 */
export interface NormalizedArtifact {
  format: ArtifactFormat;
  contractName: string;
  abi: AbiElement[];
  bytecode: string;
  deployedBytecode: string;
  /** Immutable references from Foundry (exact positions) */
  immutableReferences: ImmutableReference[] | undefined;
  /** Pre-computed method identifiers from Foundry (selector -> signature) */
  methodIdentifiers: Map<string, string> | undefined;
}

/**
 * @deprecated Use NormalizedArtifact instead. Kept for backward compatibility.
 */
export interface ArtifactJson {
  _format?: string;
  contractName: string;
  sourceName?: string;
  abi: AbiElement[];
  bytecode: string;
  deployedBytecode: string;
  linkReferences?: Record<string, unknown>;
  deployedLinkReferences?: Record<string, unknown>;
}

export interface AbiElement {
  type: string;
  name?: string;
  inputs?: AbiInput[];
  outputs?: AbiInput[];
  stateMutability?: string;
  anonymous?: boolean;
  indexed?: boolean;
}

export interface AbiInput {
  internalType: string;
  name: string;
  type: string;
  indexed?: boolean;
  components?: AbiInput[];
}

export type VerificationStatus = "pass" | "fail" | "warn" | "skip";

export interface BytecodeComparisonResult {
  status: VerificationStatus;
  message: string;
  localBytecodeLength: number | undefined;
  remoteBytecodeLength: number | undefined;
  matchPercentage: number | undefined;
  differences: BytecodeDifference[] | undefined;
  immutableDifferences: ImmutableDifference[] | undefined;
  onlyImmutablesDiffer: boolean | undefined;
}

export interface BytecodeDifference {
  position: number;
  localByte: string;
  remoteByte: string;
}

export interface AbiComparisonResult {
  status: VerificationStatus;
  message: string;
  localSelectors?: string[];
  remoteSelectors?: string[];
  missingSelectors?: string[];
  extraSelectors?: string[];
}

export interface ContractVerificationResult {
  contract: ContractConfig;
  chain: ChainConfig;
  bytecodeResult?: BytecodeComparisonResult;
  abiResult?: AbiComparisonResult;
  stateResult?: StateVerificationResult;
  error?: string;
}

export interface VerificationSummary {
  total: number;
  passed: number;
  failed: number;
  warnings: number;
  skipped: number;
  results: ContractVerificationResult[];
}

export interface CliOptions {
  config: string;
  verbose: boolean;
  contract: string | undefined;
  chain: string | undefined;
  skipBytecode: boolean;
  skipAbi: boolean;
  skipState: boolean;
}
