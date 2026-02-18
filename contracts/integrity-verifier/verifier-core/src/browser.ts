/**
 * Browser-compatible exports for @consensys/linea-contract-integrity-verifier
 *
 * This entry point excludes Node.js-only functions (loadArtifact, loadStorageSchema, loadConfig)
 * that depend on the 'fs' module.
 *
 * Use this when bundling for browser environments or static export.
 *
 * @packageDocumentation
 */

// Adapter interface (for implementers)
export type { CryptoAdapter, Web3Adapter, Web3AdapterOptions } from "./adapter";

// Main Verifier class (browser-safe version)
export { Verifier, BrowserVerifier } from "./verifier-browser";
export type { VerifyOptions, VerificationContent } from "./verifier-browser";

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
} from "./utils/bytecode";

// Browser-compatible ABI utilities (excluding loadArtifact which uses fs)
export {
  parseArtifact,
  detectArtifactFormat,
  extractSelectorsFromAbi,
  extractSelectorsFromArtifact,
  compareSelectors,
} from "./utils/abi";

// Browser-compatible storage utilities (excluding loadStorageSchema which uses fs)
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

// Comparison utilities
export {
  formatValue,
  formatForDisplay,
  compareValues,
  isNumericString,
  normalizeForComparison,
} from "./utils/comparison";
export type { ComparisonOperator } from "./utils/comparison";

// Markdown config parsing (browser-compatible)
export { parseMarkdownConfig } from "./utils/markdown-config";

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
} from "./constants";

// All types (types don't cause bundling issues)
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
} from "./types";
