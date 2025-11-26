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
 *   - rebalanceToleranceBps: Rebalance tolerance in basis points
 *   - maxValidatorWithdrawalRequestsPerTransaction: Maximum withdrawal requests per transaction
 *   - minWithdrawalThresholdEth: Minimum withdrawal threshold in ETH
 *   - reporting: Reporting configuration including shouldSubmitVaultReport, minPositiveYieldToReportWei, and minUnpaidLidoProtocolFeesToReportYieldWei
 *   - web3signer: Web3Signer URL, public key (address or secp pubkey compressed/uncompressed), keystore, truststore, and TLS settings
 *   - loggerOptions: Winston logger configuration with console transport
 */
export const toClientConfig = (env: FlattenedConfigSchema) => ({
  dataSources: {
    chainId: env.CHAIN_ID,
    l1RpcUrl: env.L1_RPC_URL,
    beaconChainRpcUrl: env.BEACON_CHAIN_RPC_URL,
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
  },
  rebalanceToleranceBps: env.REBALANCE_TOLERANCE_BPS,
  maxValidatorWithdrawalRequestsPerTransaction: env.MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION,
  minWithdrawalThresholdEth: env.MIN_WITHDRAWAL_THRESHOLD_ETH,
  reporting: {
    shouldSubmitVaultReport: env.SHOULD_SUBMIT_VAULT_REPORT,
    minPositiveYieldToReportWei: env.MIN_POSITIVE_YIELD_TO_REPORT_WEI,
    minUnpaidLidoProtocolFeesToReportYieldWei: env.MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI,
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
    level: "info",
    transports: [new transports.Console()],
  },
});

/**
 * Type representing the bootstrap configuration for the Native Yield Automation Service.
 * Derived from the return type of toClientConfig function.
 */
export type NativeYieldAutomationServiceBootstrapConfig = ReturnType<typeof toClientConfig>;
