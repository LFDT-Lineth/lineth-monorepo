import { z } from "zod";
import { isAddress, getAddress, isHex } from "viem";

/** Reusable EVM address schema: validates then normalizes to checksummed form */
const Address = z
  .string()
  .refine((v) => isAddress(v), { message: "Invalid Ethereum address" })
  .transform((v) => getAddress(v)); // checksum/normalize

const Hex = z.string().refine((v) => isHex(v), { message: "Invalid Hex" });

/** Boolean schema for environment variables. Handles "true"/"false"/"1"/"0" (case-insensitive). */
const BooleanFromString = z.preprocess(
  (val) => {
    if (typeof val === "boolean") return val;
    if (typeof val === "number") return val !== 0;
    if (typeof val === "string") {
      const lower = val.toLowerCase().trim();
      if (lower === "true" || lower === "1") return true;
      if (lower === "false" || lower === "0") return false;
    }
    return val; // Trigger validation error
  },
  z.boolean({ errorMap: () => ({ message: 'Expected "true", "false", "1", "0", or boolean.' }) }),
);

export const configSchema = z
  .object({
    /** Ethereum chain ID for the L1 network (e.g., 1 for mainnet, 560048 for hoodi).
     * Currently supports mainnet.id and hoodi.id - other chain IDs will throw an error during initialization.
     */
    CHAIN_ID: z.coerce.number().int().positive(),
    // RPC endpoint URL for the L1 Ethereum blockchain. Must be a valid HTTP/HTTPS URL.
    L1_RPC_URL: z.string().url(),
    // Beacon chain API endpoint URL for Ethereum 2.0 consensus layer.
    // See API documentation - https://ethereum.github.io/beacon-APIs/
    BEACON_CHAIN_RPC_URL: z.string().url(),
    // GraphQL endpoint URL for Consensys Staking API. Expected to require OAuth2 token.
    STAKING_GRAPHQL_URL: z.string().url(),
    // IPFS gateway base URL for retrieving Lido StakingVault report data.
    // Report CIDs are recorded on-chain in the LazyOracle.sol contract
    IPFS_BASE_URL: z.string().url(),
    // OAuth2 token endpoint URL for Consensys Staking API authentication.
    CONSENSYS_STAKING_OAUTH2_TOKEN_ENDPOINT: z.string().url(),
    // OAuth2 client ID for Consensys Staking API authentication.
    CONSENSYS_STAKING_OAUTH2_CLIENT_ID: z.string().min(1),
    /** OAuth2 client secret for Consensys Staking API authentication.
     * Must be kept secure - used with CONSENSYS_STAKING_OAUTH2_CLIENT_ID to authenticate
     * and obtain access tokens for GraphQL API requests.
     */
    CONSENSYS_STAKING_OAUTH2_CLIENT_SECRET: z.string().min(1),
    /** OAuth2 audience claim for Consensys Staking API authentication.
     * Specifies the intended recipient of the access token. Used together with client ID
     * and secret to obtain properly scoped access tokens.
     */
    CONSENSYS_STAKING_OAUTH2_AUDIENCE: z.string().min(1),
    // Address of the Linea Rollup contract.
    LINEA_ROLLUP_ADDRESS: Address,
    // Address of the Lido LazyOracle contract.
    LAZY_ORACLE_ADDRESS: Address,
    // Address of the Lido VaultHub contract.
    VAULT_HUB_ADDRESS: Address,
    // Address of the Linea YieldManager contract.
    YIELD_MANAGER_ADDRESS: Address,
    // Address of the LidoStVaultYieldProvider contract.
    LIDO_YIELD_PROVIDER_ADDRESS: Address,
    // Address of the STETH contract.
    STETH_ADDRESS: Address,
    // L2 address that receives yield distributions.
    L2_YIELD_RECIPIENT: Address,
    // Polling interval in milliseconds for watching blockchain events.
    TRIGGER_EVENT_POLL_INTERVAL_MS: z.coerce.number().int().positive(),
    // Maximum idle duration (in milliseconds) before automatically executing pending operations.
    TRIGGER_MAX_INACTION_MS: z.coerce.number().int().positive(),
    // Retry delay in milliseconds between contract read attempts after failures.
    CONTRACT_READ_RETRY_TIME_MS: z.coerce.number().int().positive(),
    // Polling interval in milliseconds for updating gauge metrics from various data sources.
    GAUGE_METRICS_POLL_INTERVAL_MS: z.coerce.number().int().positive(),
    // Whether to submit the vault accounting report. Can set to false if we expect other actors to submit.
    SHOULD_SUBMIT_VAULT_REPORT: BooleanFromString,
    // Whether to report yield. Can set to false to disable yield reporting entirely (e.g., during maintenance or when other actors are handling yield reporting).
    SHOULD_REPORT_YIELD: BooleanFromString,
    // Whether to unpause staking when conditions are met. Can set to false to disable automatic unpause of staking operations.
    IS_UNPAUSE_STAKING_ENABLED: BooleanFromString,
    /** Minimum difference between peeked negative yield and on-state negative yield (in wei) required before triggering a yield report.
     * Yield reporting will proceed if this threshold is met.
     * The difference is calculated as: peekedNegativeYield - onStateNegativeYield.
     * This prevents gas-inefficient transactions for very small negative yield changes.
     */
    MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: z
      .union([z.string(), z.number(), z.bigint()])
      .transform((val) => BigInt(val))
      .refine((v) => v >= 0n, { message: "Must be nonnegative" }),
    /** Number of processing cycles between forced yield reports.
     * Yield will be reported every N cycles regardless of threshold checks.
     * This ensures periodic yield reporting even when thresholds aren't met.
     */
    CYCLES_PER_YIELD_REPORT: z.coerce.number().int().positive(),
    /** Rebalance tolerance amount (in wei) used as an absolute tolerance band for rebalancing decisions.
     * Rebalancing occurs only when the L1 Message Service balance deviates from the effective
     * target withdrawal reserve by more than this tolerance amount (either above or below).
     * Prevents unnecessary rebalancing operations for small fluctuations.
     */
    REBALANCE_TOLERANCE_AMOUNT_WEI: z
      .union([z.string(), z.number(), z.bigint()])
      .transform((val) => BigInt(val))
      .refine((v) => v >= 0n, { message: "Must be nonnegative" }),
    // Maximum number of validator withdrawal requests that will be batched in a single transaction.
    MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION: z.coerce.number().int().positive(),
    /**
     * The available withdrawal balance must exceed this amount before any withdrawal operation proceeds.
     * This prevents gas-inefficient transactions for very small amounts.
     */
    MIN_WITHDRAWAL_THRESHOLD_ETH: z
      .union([z.string(), z.number(), z.bigint()])
      .transform((val) => BigInt(val))
      .refine((v) => v >= 0n, { message: "Must be nonnegative" }),
    /** Staking rebalance quota as basis points (bps) of Total System Balance (TSB).
     * The quota is calculated as a percentage of TSB over a rolling window of cycles.
     * 100 bps = 1%, 1800 bps = 18%, 10000 bps = 100%.
     * Used to mitigate whale-driven reserve depletion risk by limiting cumulative deposits.
     * Valid range: 0 to 10000 (0% to 100%).
     */
    STAKING_REBALANCE_QUOTA_BPS: z.coerce.number().int().min(0).max(10000),
    /** Number of cycles in the rolling window for staking rebalance quota tracking.
     * Each cycle corresponds to a YieldReportingProcessor.process() call.
     * The quota service tracks cumulative deposits over this many cycles.
     * Set to 0 to disable the quota mechanism entirely - all rebalance amounts will pass through without quota enforcement.
     */
    STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: z.coerce.number().int().min(0),
    /** Web3Signer service URL for transaction signing.
     * The service signs transactions using the key specified by WEB3SIGNER_PUBLIC_KEY.
     * Must be a valid HTTPS (not HTTP) URL.
     */
    WEB3SIGNER_URL: z.string().url(),
    /** Secp256k1 public key (uncompressed, 64 bytes) for Web3Signer transaction signing.
     * Used by Web3SignerClientAdapter to identify which key to use for signing transactions.
     * Format: 64 hex characters (128 hex digits) representing the uncompressed public key
     * without the 0x04 prefix. Optional 0x prefix is accepted. Example: "a1b2c3..." or "0xa1b2c3...".
     * This corresponds to the signing key stored in the Web3Signer keystore.
     */
    WEB3SIGNER_PUBLIC_KEY: Hex.refine(
      (v) => /^(?:0x)?[a-fA-F0-9]{128}$/.test(v), // uncompressed pubkey (64 bytes, without ...04-prefix, optional 0x prefix)
      "Expected secp256k1 public key (uncompressed, without 0x04 prefix).",
    ),
    /** File path to the Web3Signer keystore file.
     * Keystore = Who am I?
     * Contains the client’s private key and certificate used to authenticate itself
     * to the Web3Signer server during mutual TLS (mTLS) connections.
     */
    WEB3SIGNER_KEYSTORE_PATH: z.string().min(1),
    // Passphrase to decrypt the Web3Signer keystore file.
    WEB3SIGNER_KEYSTORE_PASSPHRASE: z.string().min(1),
    /** Path to the Web3Signer truststore file.
     * Truststore = Who do I trust?
     * Contains trusted CA certificates for verifying the Web3Signer server's TLS certificate.
     */
    WEB3SIGNER_TRUSTSTORE_PATH: z.string().min(1),
    // Passphrase to access the Web3Signer truststore file.
    WEB3SIGNER_TRUSTSTORE_PASSPHRASE: z.string().min(1),
    // Note: Doesn't currently do anything. Implementation currently only supports HTTPS anyway.
    WEB3SIGNER_TLS_ENABLED: BooleanFromString,
    /** Port number for the metrics API HTTP server.
     * Used to expose metrics endpoints for monitoring and observability.
     * Must be between 1024 and 49000 (inclusive) to avoid system ports and common application ports.
     */
    API_PORT: z.coerce.number().int().min(1024).max(49000),
    /** Winston logger level for controlling log verbosity.
     * Valid values: "error", "warn", "info", "verbose", "debug", "silly".
     * Defaults to "info" if not specified.
     */
    LOG_LEVEL: z
      .enum(["error", "warn", "info", "verbose", "debug", "silly"], {
        errorMap: () => ({ message: "LOG_LEVEL must be one of: error, warn, info, verbose, debug, silly" }),
      })
      .optional(),
  })
  .strip();

export type FlattenedConfigSchema = z.infer<typeof configSchema>;
