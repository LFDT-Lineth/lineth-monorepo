import { toClientConfig } from "../config.js";
import { configSchema } from "../config.schema.js";
import { transports } from "winston";

const createValidEnv = () => ({
  CHAIN_ID: "11155111",
  L1_RPC_URL: "https://rpc.linea.build",
  BEACON_CHAIN_RPC_URL: "https://beacon.linea.build",
  STAKING_GRAPHQL_URL: "https://staking.linea.build/graphql",
  IPFS_BASE_URL: "https://ipfs.linea.build",
  CONSENSYS_STAKING_OAUTH2_TOKEN_ENDPOINT: "https://auth.linea.build/token",
  CONSENSYS_STAKING_OAUTH2_CLIENT_ID: "client-id",
  CONSENSYS_STAKING_OAUTH2_CLIENT_SECRET: "client-secret",
  CONSENSYS_STAKING_OAUTH2_AUDIENCE: "audience",
  LINEA_ROLLUP_ADDRESS: "0x1111111111111111111111111111111111111111",
  LAZY_ORACLE_ADDRESS: "0x2222222222222222222222222222222222222222",
  VAULT_HUB_ADDRESS: "0x3333333333333333333333333333333333333333",
  YIELD_MANAGER_ADDRESS: "0x4444444444444444444444444444444444444444",
  LIDO_YIELD_PROVIDER_ADDRESS: "0x5555555555555555555555555555555555555555",
  L2_YIELD_RECIPIENT: "0x7777777777777777777777777777777777777777",
  TRIGGER_EVENT_POLL_INTERVAL_MS: "1000",
  TRIGGER_MAX_INACTION_MS: "5000",
  CONTRACT_READ_RETRY_TIME_MS: "250",
  REBALANCE_TOLERANCE_BPS: "500",
  MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION: "16",
  MIN_WITHDRAWAL_THRESHOLD_ETH: "42",
  WEB3SIGNER_URL: "https://web3signer.linea.build",
  WEB3SIGNER_PUBLIC_KEY: `0x${"b".repeat(128)}`,
  WEB3SIGNER_KEYSTORE_PATH: "/path/to/keystore",
  WEB3SIGNER_KEYSTORE_PASSPHRASE: "keystore-pass",
  WEB3SIGNER_TRUSTSTORE_PATH: "/path/to/truststore",
  WEB3SIGNER_TRUSTSTORE_PASSPHRASE: "truststore-pass",
  WEB3SIGNER_TLS_ENABLED: "true",
  API_PORT: "3000",
  SHOULD_SUBMIT_VAULT_REPORT: "true",
  MIN_POSITIVE_YIELD_TO_REPORT_WEI: "1000000000000000000",
  MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI: "500000000000000000",
});

describe("toClientConfig", () => {
  it("maps a validated environment into bootstrap config", () => {
    const env = configSchema.parse(createValidEnv());

    const config = toClientConfig(env);

    expect(config).toMatchObject({
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
          pollIntervalMs: env.TRIGGER_EVENT_POLL_INTERVAL_MS,
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
        publicKey: env.WEB3SIGNER_PUBLIC_KEY,
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
    });
    expect(config.loggerOptions.level).toBe("info");
    expect(config.loggerOptions.transports).toHaveLength(1);
    expect(config.loggerOptions.transports[0]).toBeInstanceOf(transports.Console);
  });
});
