/**
 * @consensys/linea-contract-integrity-verifier
 *
 * A tool to verify deployed smart contract integrity (bytecode, ABI, and state)
 * against local artifact files.
 *
 * @packageDocumentation
 */

// Adapter interface (for implementers)
export type { CryptoAdapter, Web3Adapter, Web3AdapterOptions } from "./adapter";

// Main Verifier class
export { Verifier, printSummary } from "./verifier";
export type { VerifyOptions, VerificationContent } from "./verifier";

// CLI Runner (for adapter packages)
export { runCli, parseCliArgs, printUsage, truncateValue } from "./cli-runner";
export type { CliOptions, CliRunnerConfig } from "./cli-runner";

// Config loading
export { loadConfig, checkArtifactExists } from "./config";

// Pure utility exports (no adapter needed)
export {
  stripCborMetadata,
  compareBytecode,
  extractSelectorsFromBytecode,
  validateImmutablesAgainstArgs,
  verifyImmutableValues,
  definitiveCompareBytecode,
  groupImmutableDifferences,
  formatGroupedImmutables,
  linkLibraries,
  detectUnlinkedLibraries,
  verifyLinkedLibraries,
} from "./utils/bytecode";

// ABI utilities (require adapter for selector computation)
export {
  parseArtifact,
  detectArtifactFormat,
  extractSelectorsFromAbi,
  extractSelectorsFromArtifact,
  compareSelectors,
} from "./utils/abi";

// Node.js-only ABI utilities (uses fs)
export { loadArtifact } from "./utils/abi-node";

// Storage utilities (require adapter for hashing and RPC)
export {
  calculateErc7201BaseSlot,
  readStorageSlot,
  decodeSlotValue,
  verifySlot,
  verifyNamespace,
  verifyStoragePath,
  parseStorageSchema,
  parsePath,
  computeSlot,
} from "./utils/storage";

// Node.js-only storage utilities (uses fs)
export { loadStorageSchema } from "./utils/storage-node";

// Comparison utilities
export {
  formatValue,
  formatForDisplay,
  compareValues,
  isNumericString,
  normalizeForComparison,
} from "./utils/comparison";
export type { ComparisonOperator } from "./utils/comparison";

// Validation utilities
export {
  assertNonNullish,
  assertNonEmpty,
  assertValidAddress,
  assertValidHex,
  assertValidSlot,
  isValidAddress,
  isValidHexString,
  isValidSlot,
  isObject,
  isNonEmptyArray,
} from "./utils/validation";

// Error handling utilities
export { formatError, createErrorResult } from "./utils/errors";
export type { ErrorResult } from "./utils/errors";

// Hex utilities
export { hexToBytes, getSolidityTypeSize, normalizeHex, isDecimalString } from "./utils/hex";

// Markdown config parsing
export { parseMarkdownConfig } from "./utils/markdown-config";

// Tools (require CryptoAdapter)
export {
  generateSchema,
  parseSoliditySource,
  mergeSchemas,
  calculateErc7201BaseSlot as calculateErc7201BaseSlotWithAdapter,
} from "./tools";
export type { Schema, StructDef, FieldDef, SchemaGeneratorOptions, ParseResult } from "./tools";

// Shared constants
export {
  // EIP-1967 slots
  EIP1967_IMPLEMENTATION_SLOT,
  EIP1967_ADMIN_SLOT,
  EIP1967_BEACON_SLOT,
  // OpenZeppelin Initializable (versioned)
  OZ_V4_INITIALIZABLE,
  OZ_V5_INITIALIZABLE,
  /** @deprecated Use OZ_V4_INITIALIZABLE.SLOT instead */
  OZ_INITIALIZED_SLOT,
  // ERC-7201
  ERC7201_NAMESPACE_PREFIX,
  KNOWN_NAMESPACES,
  // Sepolia addresses
  SEPOLIA_LINEA_ROLLUP_PROXY,
  SEPOLIA_LINEA_ROLLUP_IMPLEMENTATION,
  SEPOLIA_YIELD_MANAGER,
  SEPOLIA_SAFE_ADDRESS,
  // Role hashes
  ROLE_HASHES,
  // Chain IDs
  CHAIN_IDS,
  // RPC env vars
  RPC_ENV_VARS,
  // Special addresses
  ZERO_ADDRESS,
  DEAD_ADDRESS,
  // Bytecode patterns
  CBOR_METADATA_MARKER,
  IPFS_HASH_PREFIX,
  // Contract versions
  CONTRACT_VERSIONS,
  // Hex and byte constants
  HEX_PREFIX_LENGTH,
  HEX_CHARS_PER_BYTE,
  BYTES_PER_STORAGE_SLOT,
  HEX_CHARS_PER_STORAGE_SLOT,
  ADDRESS_HEX_CHARS,
  ADDRESS_BYTES,
  SELECTOR_HEX_CHARS,
  // Bytecode comparison thresholds
  BYTECODE_MATCH_THRESHOLD_PERCENT,
} from "./constants";

// All types
export type {
  // Config types
  VerifierConfig,
  ChainConfig,
  ContractConfig,
  // Verification result types
  VerificationSummary,
  ContractVerificationResult,
  BytecodeComparisonResult,
  AbiComparisonResult,
  StateVerificationResult,
  VerificationStatus,
  // State verification config types
  StateVerificationConfig,
  ViewCallConfig,
  SlotConfig,
  SlotType,
  NamespaceConfig,
  NamespaceVariable,
  StoragePathConfig,
  // State verification result types
  ViewCallResult,
  SlotResult,
  NamespaceResult,
  StoragePathResult,
  // Artifact types
  ArtifactFormat,
  NormalizedArtifact,
  HardhatArtifact,
  FoundryArtifact,
  AbiElement,
  AbiInput,
  // Storage schema types
  StorageSchema,
  StorageStructDef,
  StorageFieldDef,
  SolidityType,
  // Bytecode types
  BytecodeDifference,
  ImmutableDifference,
  ImmutableReference,
  // Immutable values types
  ImmutableValuesResult,
  ImmutableValueResult,
  DefinitiveBytecodeResult,
  GroupedImmutableDifference,
  // Linked library types
  LinkReference,
  DeployedLinkReferences,
  LinkedLibraryResult,
} from "./types";
