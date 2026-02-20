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
   * Named immutable values to verify against deployed bytecode.
   * Maps Solidity immutable variable names to expected values.
   * This is the recommended approach for contracts with mixed constructor args
   * (some become immutables, some go to storage).
   *
   * Example:
   * ```json
   * "immutableValues": {
   *   "L1_MESSAGE_SERVICE": "0x1234...",
   *   "YIELD_MANAGER": "0x5678...",
   *   "MAX_AMOUNT": 1000000
   * }
   * ```
   */
  immutableValues?: Record<string, string | number | boolean | bigint>;
  /**
   * Deployed library addresses for contracts that use linked libraries.
   * Maps fully-qualified library name to its deployed address.
   * The key format is "sourcePath:LibraryName" matching Solidity's convention.
   *
   * Example:
   * ```json
   * "linkedLibraries": {
   *   "src/libraries/Mimc.sol:Mimc": "0x1234...abcd"
   * }
   * ```
   */
  linkedLibraries?: Record<string, string>;
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

/**
 * Solidity primitive types for storage slot verification.
 * Supports all valid Solidity value types:
 * - address: 20 bytes
 * - bool: 1 byte
 * - uint8 to uint256 (in 8-bit increments): 1-32 bytes
 * - int8 to int256 (in 8-bit increments): 1-32 bytes
 * - bytes1 to bytes32: 1-32 bytes
 */
export type SlotType =
  | "address"
  | "bool"
  // All uint types (8-bit increments)
  | "uint8"
  | "uint16"
  | "uint24"
  | "uint32"
  | "uint40"
  | "uint48"
  | "uint56"
  | "uint64"
  | "uint72"
  | "uint80"
  | "uint88"
  | "uint96"
  | "uint104"
  | "uint112"
  | "uint120"
  | "uint128"
  | "uint136"
  | "uint144"
  | "uint152"
  | "uint160"
  | "uint168"
  | "uint176"
  | "uint184"
  | "uint192"
  | "uint200"
  | "uint208"
  | "uint216"
  | "uint224"
  | "uint232"
  | "uint240"
  | "uint248"
  | "uint256"
  // All int types (8-bit increments)
  | "int8"
  | "int16"
  | "int24"
  | "int32"
  | "int40"
  | "int48"
  | "int56"
  | "int64"
  | "int72"
  | "int80"
  | "int88"
  | "int96"
  | "int104"
  | "int112"
  | "int120"
  | "int128"
  | "int136"
  | "int144"
  | "int152"
  | "int160"
  | "int168"
  | "int176"
  | "int184"
  | "int192"
  | "int200"
  | "int208"
  | "int216"
  | "int224"
  | "int232"
  | "int240"
  | "int248"
  | "int256"
  // All bytes types (1-32)
  | "bytes1"
  | "bytes2"
  | "bytes3"
  | "bytes4"
  | "bytes5"
  | "bytes6"
  | "bytes7"
  | "bytes8"
  | "bytes9"
  | "bytes10"
  | "bytes11"
  | "bytes12"
  | "bytes13"
  | "bytes14"
  | "bytes15"
  | "bytes16"
  | "bytes17"
  | "bytes18"
  | "bytes19"
  | "bytes20"
  | "bytes21"
  | "bytes22"
  | "bytes23"
  | "bytes24"
  | "bytes25"
  | "bytes26"
  | "bytes27"
  | "bytes28"
  | "bytes29"
  | "bytes30"
  | "bytes31"
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

/**
 * Solidity types for storage schema definitions.
 * Includes all primitive types plus arrays and mappings.
 */
export type SolidityType =
  | "address"
  | "bool"
  | "string"
  | "bytes"
  // All uint types (8-bit increments)
  | "uint8"
  | "uint16"
  | "uint24"
  | "uint32"
  | "uint40"
  | "uint48"
  | "uint56"
  | "uint64"
  | "uint72"
  | "uint80"
  | "uint88"
  | "uint96"
  | "uint104"
  | "uint112"
  | "uint120"
  | "uint128"
  | "uint136"
  | "uint144"
  | "uint152"
  | "uint160"
  | "uint168"
  | "uint176"
  | "uint184"
  | "uint192"
  | "uint200"
  | "uint208"
  | "uint216"
  | "uint224"
  | "uint232"
  | "uint240"
  | "uint248"
  | "uint256"
  // All int types (8-bit increments)
  | "int8"
  | "int16"
  | "int24"
  | "int32"
  | "int40"
  | "int48"
  | "int56"
  | "int64"
  | "int72"
  | "int80"
  | "int88"
  | "int96"
  | "int104"
  | "int112"
  | "int120"
  | "int128"
  | "int136"
  | "int144"
  | "int152"
  | "int160"
  | "int168"
  | "int176"
  | "int184"
  | "int192"
  | "int200"
  | "int208"
  | "int216"
  | "int224"
  | "int232"
  | "int240"
  | "int248"
  | "int256"
  // All bytes types (1-32)
  | "bytes1"
  | "bytes2"
  | "bytes3"
  | "bytes4"
  | "bytes5"
  | "bytes6"
  | "bytes7"
  | "bytes8"
  | "bytes9"
  | "bytes10"
  | "bytes11"
  | "bytes12"
  | "bytes13"
  | "bytes14"
  | "bytes15"
  | "bytes16"
  | "bytes17"
  | "bytes18"
  | "bytes19"
  | "bytes20"
  | "bytes21"
  | "bytes22"
  | "bytes23"
  | "bytes24"
  | "bytes25"
  | "bytes26"
  | "bytes27"
  | "bytes28"
  | "bytes29"
  | "bytes30"
  | "bytes31"
  | "bytes32"
  // Array types
  | `${string}[]`
  // Mapping types
  | `mapping(${string} => ${string})`
  // Struct types (referenced by name)
  | string;

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

export interface LinkReference {
  start: number; // byte position in deployed bytecode
  length: number; // always 20 for library addresses
}

/**
 * Nested link references structure from Hardhat/Foundry artifacts.
 * Maps source file path -> library name -> array of byte positions.
 */
export type DeployedLinkReferences = Record<string, Record<string, LinkReference[]>>;

export interface ImmutableDifference {
  position: number;
  length: number;
  localValue: string;
  remoteValue: string;
  possibleType: string | undefined;
}

/**
 * Result of verifying a single named immutable value.
 */
export interface ImmutableValueResult {
  /** The immutable variable name */
  name: string;
  /** Expected value from config */
  expected: string;
  /** Actual value found in bytecode */
  actual: string | undefined;
  /** Verification status */
  status: VerificationStatus;
  /** Human-readable message */
  message: string;
}

/**
 * Result of verifying all named immutable values.
 */
export interface ImmutableValuesResult {
  /** Overall status */
  status: VerificationStatus;
  /** Summary message */
  message: string;
  /** Individual results for each named immutable */
  results: ImmutableValueResult[];
}

/**
 * Result of verifying a single linked library substitution.
 */
export interface LinkedLibraryResult {
  /** Fully-qualified library name ("sourcePath:LibraryName") */
  name: string;
  /** Deployed address provided in config */
  address: string;
  /** Actual address extracted from on-chain bytecode (if available) */
  actualAddress: string | undefined;
  /** Byte positions where the address was substituted */
  positions: number[];
  /** Verification status */
  status: VerificationStatus;
  /** Human-readable message */
  message: string;
}

/**
 * Result of definitive bytecode comparison after immutable substitution.
 * This provides 100% confidence by substituting known immutable values
 * into the local bytecode and comparing byte-for-byte with remote.
 */
export interface DefinitiveBytecodeResult {
  /** Whether bytecode matches exactly after immutable substitution */
  exactMatch: boolean;
  /** Verification status */
  status: VerificationStatus;
  /** Human-readable message */
  message: string;
  /** Number of immutables substituted */
  immutablesSubstituted: number;
  /** Hash of local bytecode after substitution (for debugging) */
  localHashAfterSubstitution?: string;
  /** Hash of remote bytecode (for debugging) */
  remoteHash?: string;
}

/**
 * Grouped immutable difference - groups detected fragments by their parent immutable reference.
 */
export interface GroupedImmutableDifference {
  /** Index number for display (1-based) */
  index: number;
  /** Start position of the immutable reference (from artifact) */
  refStart: number;
  /** Length of the immutable reference (from artifact) */
  refLength: number;
  /** The full value reconstructed from fragments or read directly */
  fullValue: string;
  /** Whether this immutable was detected as fragmented */
  isFragmented: boolean;
  /** The individual fragments (if fragmented) */
  fragments: ImmutableDifference[];
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
  linkReferences?: DeployedLinkReferences;
  deployedLinkReferences?: DeployedLinkReferences;
}

/**
 * Foundry artifact format (forge build output)
 */
export interface FoundryArtifact {
  abi: AbiElement[];
  bytecode: {
    object: string;
    sourceMap?: string;
    linkReferences?: DeployedLinkReferences;
  };
  deployedBytecode: {
    object: string;
    sourceMap?: string;
    linkReferences?: DeployedLinkReferences;
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
  /** Deployed link references for external library linking */
  deployedLinkReferences: DeployedLinkReferences | undefined;
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
  linkReferences?: DeployedLinkReferences;
  deployedLinkReferences?: DeployedLinkReferences;
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
  immutableValuesResult?: ImmutableValuesResult;
  /** Definitive bytecode verification (100% confidence, no ambiguity) */
  definitiveResult?: DefinitiveBytecodeResult;
  /** Grouped immutable differences for better display (shows fragmented immutables) */
  groupedImmutables?: GroupedImmutableDifference[];
  /** Linked library verification results */
  linkedLibrariesResult?: LinkedLibraryResult[];
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
