/**
 * @consensys/linea-contract-integrity-verifier
 *
 * A tool to verify deployed smart contract integrity (bytecode, ABI, and state)
 * against local artifact files.
 *
 * @packageDocumentation
 */

// Adapter interface (for implementers)
export type { Web3Adapter, Web3AdapterOptions } from "./adapter";

// Main Verifier class
export { Verifier, printSummary } from "./verifier";
export type { VerifyOptions } from "./verifier";

// Config loading
export { loadConfig, checkArtifactExists } from "./config";

// Pure utility exports (no adapter needed)
export {
  stripCborMetadata,
  compareBytecode,
  extractSelectorsFromBytecode,
  validateImmutablesAgainstArgs,
} from "./utils/bytecode";

// ABI utilities (require adapter for selector computation)
export {
  loadArtifact,
  detectArtifactFormat,
  extractSelectorsFromAbi,
  extractSelectorsFromArtifact,
  compareSelectors,
} from "./utils/abi";

// Storage utilities (require adapter for hashing and RPC)
export {
  calculateErc7201BaseSlot,
  calculateErc7201Slot,
  readStorageSlot,
  decodeSlotValue,
  verifySlot,
  verifyNamespace,
  verifyStoragePath,
  loadStorageSchema,
  parsePath,
  computeSlot,
} from "./utils/storage";

// Markdown config parsing
export { parseMarkdownConfig } from "./utils/markdown-config";

// Shared constants
export {
  // EIP-1967 slots
  EIP1967_IMPLEMENTATION_SLOT,
  EIP1967_ADMIN_SLOT,
  EIP1967_BEACON_SLOT,
  // OZ slots
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

// All types
export type {
  // Config types
  VerifierConfig,
  ChainConfig,
  ContractConfig,
  CliOptions,
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
} from "./types";
