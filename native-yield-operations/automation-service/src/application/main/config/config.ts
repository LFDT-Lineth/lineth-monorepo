import { Hex } from "viem";
import { FlattenedConfigSchema } from "./config.schema.js";
import { transports } from "winston";

/**
 * Converts a flattened configuration schema to a client configuration object.
 * Transforms environment variables into a structured configuration with nested objects
 * for data sources, OAuth2 settings, contract addresses, timing settings, and Web3Signer configuration.
 *
 * @param {FlattenedConfigSchema} env - The flattened configuration schema containing environment variables.
 * @returns {Object} A client configuration object with the following structure:
 *   - dataSources: Chain ID, RPC URLs, GraphQL URL, and IPFS base URL
 *   - consensysStakingOAuth2: OAuth2 token endpoint and credentials
 *   - contractAddresses: All contract addresses used by the service
 *   - apiPort: Port for the metrics API server
 *   - timing: Poll intervals and retry timing configuration
 *   - rebalance: Rebalance configuration including tolerance, thresholds, and limits
 *   - reporting: Reporting configuration including shouldSubmitVaultReport, shouldReportYield, isUnpauseStakingEnabled, and minNegativeYieldDiffToReportYieldWei
 *   - web3signer: Web3Signer URL, public key (address or secp pubkey compressed/uncompressed), keystore, truststore, and TLS settings
 *   - loggerOptions: Winston logger configuration with console transport and log level from LOG_LEVEL env var (defaults to "info")
 */
export const toClientConfig = (env: FlattenedConfigSchema) => ({
  dataSources: {
    chainId: env.CHAIN_ID,
    l1RpcUrl: env.L1_RPC_URL,
    l1RpcUrlFallback: env.L1_RPC_URL_FALLBACK,
    beaconChainRpcUrl: env.BEACON_CHAIN_RPC_URL,
    referenceBeaconChainRpcUrl: env.REFERENCE_BEACON_CHAIN_RPC_URL,
    stakingGraphQLUrl: env.STAKING_GRAPHQL_URL,
    ipfsBaseUrl: env.IPFS_BASE_URL,
  },
  consensysStakingOAuth2: {
    tokenEndpoint: env.CONSENSYS_STAKING_OAUTH2_TOKEN_ENDPOINT,
    clientId: env.CONSENSYS_STAKING_OAUTH2_CLIENT_ID,
    clientSecret: env.CONSENSYS_STAKING_OAUTH2_CLIENT_SECRET,
    audience: env.CONSENSYS_STAKING_OAUTH2_AUDIENCE,
  },
  contractAddresses: {
    lineaRollupContractAddress: env.LINEA_ROLLUP_ADDRESS,
    lazyOracleAddress: env.LAZY_ORACLE_ADDRESS,
    vaultHubAddress: env.VAULT_HUB_ADDRESS,
    yieldManagerAddress: env.YIELD_MANAGER_ADDRESS,
    lidoYieldProviderAddress: env.LIDO_YIELD_PROVIDER_ADDRESS,
    stethAddress: env.STETH_ADDRESS,
    l2YieldRecipientAddress: env.L2_YIELD_RECIPIENT,
  },
  apiPort: env.API_PORT,
  timing: {
    trigger: {
      // How often we poll for the trigger event
      pollIntervalMs: env.TRIGGER_EVENT_POLL_INTERVAL_MS,
      // Max tolerated time for inaction if trigger event polling doesn't find the trigger event
      maxInactionMs: env.TRIGGER_MAX_INACTION_MS,
    },
    contractReadRetryTimeMs: env.CONTRACT_READ_RETRY_TIME_MS,
    gaugeMetricsPollIntervalMs: env.GAUGE_METRICS_POLL_INTERVAL_MS,
  },
  rebalance: {
    toleranceAmountWei: env.REBALANCE_TOLERANCE_AMOUNT_WEI,
    maxValidatorWithdrawalRequestsPerTransaction: env.MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION,
    minWithdrawalThresholdEth: env.MIN_WITHDRAWAL_THRESHOLD_ETH,
    stakingRebalanceQuotaBps: env.STAKING_REBALANCE_QUOTA_BPS,
    stakingRebalanceQuotaWindowSizeInCycles: env.STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES,
  },
  reporting: {
    shouldSubmitVaultReport: env.SHOULD_SUBMIT_VAULT_REPORT,
    shouldReportYield: env.SHOULD_REPORT_YIELD,
    isUnpauseStakingEnabled: env.IS_UNPAUSE_STAKING_ENABLED,
    minNegativeYieldDiffToReportYieldWei: env.MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI,
    cyclesPerYieldReport: env.CYCLES_PER_YIELD_REPORT,
  },
  web3signer: {
    url: env.WEB3SIGNER_URL,
    publicKey: env.WEB3SIGNER_PUBLIC_KEY as Hex, // address or secp pubkey (compressed/uncompressed)
    keystore: {
      path: env.WEB3SIGNER_KEYSTORE_PATH,
      passphrase: env.WEB3SIGNER_KEYSTORE_PASSPHRASE,
    },
    truststore: {
      path: env.WEB3SIGNER_TRUSTSTORE_PATH,
      passphrase: env.WEB3SIGNER_TRUSTSTORE_PASSPHRASE,
    },
    tlsEnabled: env.WEB3SIGNER_TLS_ENABLED,
  },
  loggerOptions: {
    level: env.LOG_LEVEL ?? "info",
    transports: [new transports.Console()],
  },
});

/**
 * Type representing the bootstrap configuration for the Native Yield Automation Service.
 * Derived from the return type of toClientConfig function.
 */
export type NativeYieldAutomationServiceBootstrapConfig = ReturnType<typeof toClientConfig>;
