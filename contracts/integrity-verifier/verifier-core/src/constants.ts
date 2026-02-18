/**
 * Contract Integrity Verifier - Shared Constants
 *
 * Common values used across the verifier packages.
 * Centralizes magic strings, addresses, and slot definitions.
 */

// ============================================================================
// EIP-1967 Proxy Storage Slots
// ============================================================================

/**
 * EIP-1967 implementation slot.
 * keccak256("eip1967.proxy.implementation") - 1
 */
export const EIP1967_IMPLEMENTATION_SLOT = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc";

/**
 * EIP-1967 admin slot.
 * keccak256("eip1967.proxy.admin") - 1
 */
export const EIP1967_ADMIN_SLOT = "0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103";

/**
 * EIP-1967 beacon slot.
 * keccak256("eip1967.proxy.beacon") - 1
 */
export const EIP1967_BEACON_SLOT = "0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50";

// ============================================================================
// OpenZeppelin Storage Slots
// ============================================================================

/**
 * OpenZeppelin v4 Initializable storage.
 * _initialized (uint8) and _initializing (bool) are packed at slot 0.
 *
 * Layout at slot 0x0:
 * - bytes 0-0: _initialized (uint8)
 * - byte 1: _initializing (bool)
 */
export const OZ_V4_INITIALIZABLE = {
  /** Storage slot for v4 Initializable */
  SLOT: "0x0",
  /** Type of _initialized in v4 */
  INITIALIZED_TYPE: "uint8" as const,
  /** Byte offset of _initializing in the packed slot */
  INITIALIZING_OFFSET: 1,
} as const;

/**
 * OpenZeppelin v5 Initializable storage (ERC-7201 namespaced).
 * Uses namespace "openzeppelin.storage.Initializable".
 *
 * Formula: keccak256(abi.encode(uint256(keccak256("openzeppelin.storage.Initializable")) - 1)) & ~bytes32(uint256(0xff))
 *
 * Layout at base slot:
 * - slot+0, bytes 0-7: _initialized (uint64)
 * - slot+0, byte 8: _initializing (bool)
 */
export const OZ_V5_INITIALIZABLE = {
  /** ERC-7201 namespace ID */
  NAMESPACE: "openzeppelin.storage.Initializable",
  /** Pre-computed ERC-7201 base slot */
  SLOT: "0xf0c57e16840df040f15088dc2f81fe391c3923bec73e23a9662efc9c229c6a00",
  /** Type of _initialized in v5 (upgraded from uint8) */
  INITIALIZED_TYPE: "uint64" as const,
  /** Byte offset of _initializing in the packed slot */
  INITIALIZING_OFFSET: 8,
} as const;

/**
 * @deprecated Use OZ_V4_INITIALIZABLE.SLOT instead. Kept for backward compatibility.
 */
export const OZ_INITIALIZED_SLOT = OZ_V4_INITIALIZABLE.SLOT;

// ============================================================================
// ERC-7201 Namespaces
// ============================================================================

/**
 * Common ERC-7201 namespace prefixes.
 */
export const ERC7201_NAMESPACE_PREFIX = "linea.storage.";

/**
 * Known storage namespaces.
 */
export const KNOWN_NAMESPACES = {
  // Linea namespaces
  YIELD_MANAGER: "linea.storage.YieldManagerStorage",
  LINEA_ROLLUP_YIELD_EXTENSION: "linea.storage.LineaRollupYieldExtensionStorage",
  // OpenZeppelin v5 namespaces
  OZ_INITIALIZABLE: "openzeppelin.storage.Initializable",
  OZ_ACCESS_CONTROL: "openzeppelin.storage.AccessControl",
  OZ_OWNABLE: "openzeppelin.storage.Ownable",
  OZ_PAUSABLE: "openzeppelin.storage.Pausable",
  OZ_REENTRANCY_GUARD: "openzeppelin.storage.ReentrancyGuard",
} as const;

// ============================================================================
// Sepolia Test Addresses
// ============================================================================

/**
 * LineaRollup proxy address on Sepolia.
 */
export const SEPOLIA_LINEA_ROLLUP_PROXY = "0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48";

/**
 * LineaRollup implementation address on Sepolia (V7).
 */
export const SEPOLIA_LINEA_ROLLUP_IMPLEMENTATION = "0xCaAa421FfCF701bEFd676a2F5d0A161CCFA5a07E";

/**
 * YieldManager address on Sepolia.
 */
export const SEPOLIA_YIELD_MANAGER = "0xafeB487DD3E3Cb0342e8CF0215987FfDc9b72c9b";

/**
 * Safe multisig on Sepolia (for role verification).
 */
export const SEPOLIA_SAFE_ADDRESS = "0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1";

// ============================================================================
// Role Hashes
// ============================================================================

/**
 * Common role hashes (keccak256 of role names).
 */
export const ROLE_HASHES = {
  /** keccak256("DEFAULT_ADMIN_ROLE") */
  DEFAULT_ADMIN: "0x0000000000000000000000000000000000000000000000000000000000000000",
  /** keccak256("OPERATOR_ROLE") */
  OPERATOR: "0x76ef52a5344b10ed112c1d48c7c06f51e919518ea6fb005f9b25b359b955e3be",
  /** keccak256("VERIFIER_SETTER_ROLE") */
  VERIFIER_SETTER: "0x220bd22ef7c53d75fe3eac0a09e90815a0c5ba4f9e8da8b039542cd3db347258",
  /** keccak256("PAUSE_NATIVE_YIELD_STAKING_ROLE") */
  PAUSE_NATIVE_YIELD_STAKING: "0xcc10d6eec3c757d645e27b3f3001a3ba52f692da0bce25fabf58c6ecaf376450",
  /** keccak256("UNPAUSE_NATIVE_YIELD_STAKING_ROLE") */
  UNPAUSE_NATIVE_YIELD_STAKING: "0x4b4665d8754e6ea0608430ef3e91c1b45c72aafe8800e289cd35f38d85361858",
} as const;

// ============================================================================
// Chain Configuration
// ============================================================================

/**
 * Chain IDs for supported networks.
 */
export const CHAIN_IDS = {
  ETHEREUM_MAINNET: 1,
  ETHEREUM_SEPOLIA: 11155111,
  ETHEREUM_HOODI: 560048,
  LINEA_MAINNET: 59144,
  LINEA_SEPOLIA: 59141,
  LOCAL: 31337,
} as const;

/**
 * Environment variable names for RPC URLs.
 */
export const RPC_ENV_VARS = {
  ETHEREUM_MAINNET: "ETHEREUM_MAINNET_RPC_URL",
  ETHEREUM_SEPOLIA: "ETHEREUM_SEPOLIA_RPC_URL",
  ETHEREUM_HOODI: "ETHEREUM_HOODI_RPC_URL",
  LINEA_MAINNET: "LINEA_MAINNET_RPC_URL",
  LINEA_SEPOLIA: "LINEA_SEPOLIA_RPC_URL",
} as const;

// ============================================================================
// Special Addresses
// ============================================================================

/**
 * Zero address constant.
 */
export const ZERO_ADDRESS = "0x0000000000000000000000000000000000000000";

/**
 * Dead address (burn address).
 */
export const DEAD_ADDRESS = "0x000000000000000000000000000000000000dEaD";

// ============================================================================
// Bytecode Patterns
// ============================================================================

/**
 * CBOR metadata marker (Solidity >=0.5.10).
 * Indicates start of CBOR-encoded metadata at end of bytecode.
 */
export const CBOR_METADATA_MARKER = "a265";

/**
 * IPFS hash prefix in CBOR metadata.
 */
export const IPFS_HASH_PREFIX = "697066735822";

// ============================================================================
// Bytecode Comparison Configuration
// ============================================================================

/**
 * CBOR metadata length bounds (in bytes).
 *
 * Typical metadata structure:
 * - IPFS hash: 34 bytes
 * - Solc version: 4 bytes
 * - CBOR encoding overhead: ~10-20 bytes
 * - Experimental/custom fields: variable
 *
 * Conservative bounds to accommodate various compiler versions and toolchains:
 * - Min: 20 bytes (smallest valid CBOR map with minimal content)
 * - Max: 500 bytes (allows for large experimental builds, extra metadata)
 *
 * Note: The CBOR marker (a1/a2) is validated separately, so these bounds
 * serve as a sanity check rather than the primary validation.
 */
export const CBOR_METADATA_MIN_LENGTH = 20;
export const CBOR_METADATA_MAX_LENGTH = 500;

/**
 * Absolute maximum metadata length for hard validation.
 * Metadata larger than this is almost certainly invalid.
 * This prevents stripping large portions of bytecode by mistake.
 */
export const CBOR_METADATA_ABSOLUTE_MAX = 1000;

/**
 * Match percentage threshold for upgrading bytecode status.
 * When match percentage is above this threshold and named immutables
 * are verified, bytecode status can be upgraded from "fail" to "pass".
 */
export const BYTECODE_MATCH_THRESHOLD_PERCENT = 90;

// ============================================================================
// Contract Versions
// ============================================================================

/**
 * Expected contract versions for verification.
 */
export const CONTRACT_VERSIONS = {
  LINEA_ROLLUP_V7: "7.0",
} as const;

// ============================================================================
// Configuration Limits
// ============================================================================

/**
 * Maximum size for markdown config files (in bytes).
 * Prevents DoS from extremely large files.
 */
export const MAX_MARKDOWN_CONFIG_SIZE = 5 * 1024 * 1024; // 5MB

// ============================================================================
// Hex and Byte Constants
// ============================================================================

/** Length of "0x" prefix in hex strings */
export const HEX_PREFIX_LENGTH = 2;

/** Number of hex characters per byte (2 hex chars = 1 byte) */
export const HEX_CHARS_PER_BYTE = 2;

/** Bytes in an EVM storage slot */
export const BYTES_PER_STORAGE_SLOT = 32;

/** Hex characters in a 32-byte storage slot (64 chars) */
export const HEX_CHARS_PER_STORAGE_SLOT = 64;

/** Hex characters in an Ethereum address (40 chars, without 0x) */
export const ADDRESS_HEX_CHARS = 40;

/** Bytes in an Ethereum address */
export const ADDRESS_BYTES = 20;

/** Hex characters in a function selector (8 chars) */
export const SELECTOR_HEX_CHARS = 8;

/** PUSH4 opcode used for function selectors in bytecode */
export const PUSH4_OPCODE = "63";

/** Null selector (all zeros) */
export const NULL_SELECTOR = "00000000";

/** Max selector (all f's) */
export const MAX_SELECTOR = "ffffffff";

// ============================================================================
// Bytecode Analysis Constants
// ============================================================================

/** Minimum bytecode length in hex chars for metadata stripping */
export const MIN_BYTECODE_HEX_LENGTH = 8;

/** Minimum bytecode hex chars after metadata stripping */
export const MIN_BYTECODE_AFTER_STRIP = 20;

/** Hex chars for metadata length indicator (2 bytes = 4 chars) */
export const METADATA_LENGTH_HEX_CHARS = 4;

/** Minimum bytecode hex chars for selector extraction */
export const MIN_BYTECODE_FOR_SELECTORS = 10;

/** Common immutable byte sizes in Solidity */
export const COMMON_IMMUTABLE_SIZES = [1, 2, 4, 8, 12, 16, 20, 32];

/** Maximum difference regions to consider as immutable-only */
export const MAX_REGIONS_FOR_IMMUTABLE_CHECK = 5;

/** Stricter threshold for immutable check */
export const MAX_REGIONS_FOR_IMMUTABLE_CHECK_STRICT = 8;

/** Maximum differences to return in comparison results */
export const MAX_DIFFERENCES_TO_RETURN = 10;

/** Maximum diff positions to include in grouped results */
export const MAX_DIFF_POSITIONS = 10;

/** Minimum fragment length for immutable grouping */
export const MIN_FRAGMENT_LENGTH = 4;

// ============================================================================
// Display Constants
// ============================================================================

/** Default maximum length for display truncation */
export const DEFAULT_MAX_DISPLAY_LENGTH = 20;

/** Summary separator line length */
export const SUMMARY_LINE_LENGTH = 50;

/** Full separator line length */
export const FULL_LINE_LENGTH = 60;

// ============================================================================
// Comparison Constants
// ============================================================================

/** Maximum extra selectors to report in ABI comparison */
export const MAX_EXTRA_SELECTORS_TO_REPORT = 10;

// ============================================================================
// Markdown Config Constants
// ============================================================================

/** Number of header rows in markdown tables */
export const TABLE_HEADER_ROWS = 2;

/** Minimum rows required in a valid markdown table */
export const MIN_TABLE_ROWS = 2;
