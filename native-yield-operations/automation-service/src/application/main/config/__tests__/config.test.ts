import { describe, it, expect } from "@jest/globals";
import { transports } from "winston";

import { toClientConfig } from "../config.js";
import { configSchema } from "../config.schema.js";

// Semantic constants
const TEST_CHAIN_ID = "11155111";
const TEST_L1_RPC_URL = "https://rpc.linea.build";
const TEST_BEACON_CHAIN_RPC_URL = "https://beacon.linea.build";
const TEST_STAKING_GRAPHQL_URL = "https://staking.linea.build/graphql";
const TEST_IPFS_BASE_URL = "https://ipfs.linea.build";
const TEST_OAUTH2_TOKEN_ENDPOINT = "https://auth.linea.build/token";
const TEST_OAUTH2_CLIENT_ID = "client-id";
const TEST_OAUTH2_CLIENT_SECRET = "client-secret";
const TEST_OAUTH2_AUDIENCE = "audience";
const TEST_LINEA_ROLLUP_ADDRESS = "0x1111111111111111111111111111111111111111";
const TEST_LAZY_ORACLE_ADDRESS = "0x2222222222222222222222222222222222222222";
const TEST_VAULT_HUB_ADDRESS = "0x3333333333333333333333333333333333333333";
const TEST_YIELD_MANAGER_ADDRESS = "0x4444444444444444444444444444444444444444";
const TEST_LIDO_YIELD_PROVIDER_ADDRESS = "0x5555555555555555555555555555555555555555";
const TEST_STETH_ADDRESS = "0x6666666666666666666666666666666666666666";
const TEST_L2_YIELD_RECIPIENT = "0x7777777777777777777777777777777777777777";
const TEST_TRIGGER_POLL_INTERVAL_MS = "1000";
const TEST_TRIGGER_MAX_INACTION_MS = "5000";
const TEST_CONTRACT_READ_RETRY_TIME_MS = "250";
const TEST_GAUGE_METRICS_POLL_INTERVAL_MS = "5000";
const TEST_REBALANCE_TOLERANCE_AMOUNT_WEI = "5000000000000000000";
const TEST_MAX_VALIDATOR_WITHDRAWAL_REQUESTS = "16";
const TEST_MIN_WITHDRAWAL_THRESHOLD_ETH = "42";
const TEST_STAKING_REBALANCE_QUOTA_BPS = "1800";
const TEST_STAKING_REBALANCE_QUOTA_WINDOW_SIZE = "24";
const TEST_WEB3SIGNER_URL = "https://web3signer.linea.build";
const TEST_WEB3SIGNER_PUBLIC_KEY = `0x${"b".repeat(128)}`;
const TEST_WEB3SIGNER_KEYSTORE_PATH = "/path/to/keystore";
const TEST_WEB3SIGNER_KEYSTORE_PASSPHRASE = "keystore-pass";
const TEST_WEB3SIGNER_TRUSTSTORE_PATH = "/path/to/truststore";
const TEST_WEB3SIGNER_TRUSTSTORE_PASSPHRASE = "truststore-pass";
const TEST_WEB3SIGNER_TLS_ENABLED = "true";
const TEST_API_PORT = "3000";
const TEST_SHOULD_SUBMIT_VAULT_REPORT = "true";
const TEST_SHOULD_REPORT_YIELD = "true";
const TEST_IS_UNPAUSE_STAKING_ENABLED = "true";
const TEST_MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI = "1000000000000000000";
const TEST_CYCLES_PER_YIELD_REPORT = "12";
const DEFAULT_LOG_LEVEL = "info";

// Factory functions
const createValidEnv = () => ({
  CHAIN_ID: TEST_CHAIN_ID,
  L1_RPC_URL: TEST_L1_RPC_URL,
  BEACON_CHAIN_RPC_URL: TEST_BEACON_CHAIN_RPC_URL,
  STAKING_GRAPHQL_URL: TEST_STAKING_GRAPHQL_URL,
  IPFS_BASE_URL: TEST_IPFS_BASE_URL,
  CONSENSYS_STAKING_OAUTH2_TOKEN_ENDPOINT: TEST_OAUTH2_TOKEN_ENDPOINT,
  CONSENSYS_STAKING_OAUTH2_CLIENT_ID: TEST_OAUTH2_CLIENT_ID,
  CONSENSYS_STAKING_OAUTH2_CLIENT_SECRET: TEST_OAUTH2_CLIENT_SECRET,
  CONSENSYS_STAKING_OAUTH2_AUDIENCE: TEST_OAUTH2_AUDIENCE,
  LINEA_ROLLUP_ADDRESS: TEST_LINEA_ROLLUP_ADDRESS,
  LAZY_ORACLE_ADDRESS: TEST_LAZY_ORACLE_ADDRESS,
  VAULT_HUB_ADDRESS: TEST_VAULT_HUB_ADDRESS,
  YIELD_MANAGER_ADDRESS: TEST_YIELD_MANAGER_ADDRESS,
  LIDO_YIELD_PROVIDER_ADDRESS: TEST_LIDO_YIELD_PROVIDER_ADDRESS,
  STETH_ADDRESS: TEST_STETH_ADDRESS,
  L2_YIELD_RECIPIENT: TEST_L2_YIELD_RECIPIENT,
  TRIGGER_EVENT_POLL_INTERVAL_MS: TEST_TRIGGER_POLL_INTERVAL_MS,
  TRIGGER_MAX_INACTION_MS: TEST_TRIGGER_MAX_INACTION_MS,
  CONTRACT_READ_RETRY_TIME_MS: TEST_CONTRACT_READ_RETRY_TIME_MS,
  GAUGE_METRICS_POLL_INTERVAL_MS: TEST_GAUGE_METRICS_POLL_INTERVAL_MS,
  REBALANCE_TOLERANCE_AMOUNT_WEI: TEST_REBALANCE_TOLERANCE_AMOUNT_WEI,
  MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION: TEST_MAX_VALIDATOR_WITHDRAWAL_REQUESTS,
  MIN_WITHDRAWAL_THRESHOLD_ETH: TEST_MIN_WITHDRAWAL_THRESHOLD_ETH,
  STAKING_REBALANCE_QUOTA_BPS: TEST_STAKING_REBALANCE_QUOTA_BPS,
  STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: TEST_STAKING_REBALANCE_QUOTA_WINDOW_SIZE,
  WEB3SIGNER_URL: TEST_WEB3SIGNER_URL,
  WEB3SIGNER_PUBLIC_KEY: TEST_WEB3SIGNER_PUBLIC_KEY,
  WEB3SIGNER_KEYSTORE_PATH: TEST_WEB3SIGNER_KEYSTORE_PATH,
  WEB3SIGNER_KEYSTORE_PASSPHRASE: TEST_WEB3SIGNER_KEYSTORE_PASSPHRASE,
  WEB3SIGNER_TRUSTSTORE_PATH: TEST_WEB3SIGNER_TRUSTSTORE_PATH,
  WEB3SIGNER_TRUSTSTORE_PASSPHRASE: TEST_WEB3SIGNER_TRUSTSTORE_PASSPHRASE,
  WEB3SIGNER_TLS_ENABLED: TEST_WEB3SIGNER_TLS_ENABLED,
  API_PORT: TEST_API_PORT,
  SHOULD_SUBMIT_VAULT_REPORT: TEST_SHOULD_SUBMIT_VAULT_REPORT,
  SHOULD_REPORT_YIELD: TEST_SHOULD_REPORT_YIELD,
  IS_UNPAUSE_STAKING_ENABLED: TEST_IS_UNPAUSE_STAKING_ENABLED,
  MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: TEST_MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI,
  CYCLES_PER_YIELD_REPORT: TEST_CYCLES_PER_YIELD_REPORT,
});

const createEnvWithLogLevel = (logLevel: string) => ({
  ...createValidEnv(),
  LOG_LEVEL: logLevel,
});

describe("toClientConfig", () => {
  it("maps validated environment to bootstrap config", () => {
    // Arrange
    const env = configSchema.parse(createValidEnv());

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config).toMatchObject({
      dataSources: {
        chainId: expect.any(Number),
        l1RpcUrl: TEST_L1_RPC_URL,
        beaconChainRpcUrl: TEST_BEACON_CHAIN_RPC_URL,
        stakingGraphQLUrl: TEST_STAKING_GRAPHQL_URL,
        ipfsBaseUrl: TEST_IPFS_BASE_URL,
      },
      consensysStakingOAuth2: {
        tokenEndpoint: TEST_OAUTH2_TOKEN_ENDPOINT,
        clientId: TEST_OAUTH2_CLIENT_ID,
        clientSecret: TEST_OAUTH2_CLIENT_SECRET,
        audience: TEST_OAUTH2_AUDIENCE,
      },
      contractAddresses: {
        lineaRollupContractAddress: expect.any(String),
        lazyOracleAddress: expect.any(String),
        vaultHubAddress: expect.any(String),
        yieldManagerAddress: expect.any(String),
        lidoYieldProviderAddress: expect.any(String),
        stethAddress: expect.any(String),
        l2YieldRecipientAddress: expect.any(String),
      },
      apiPort: expect.any(Number),
      timing: {
        trigger: {
          pollIntervalMs: expect.any(Number),
          maxInactionMs: expect.any(Number),
        },
        contractReadRetryTimeMs: expect.any(Number),
        gaugeMetricsPollIntervalMs: expect.any(Number),
      },
      rebalance: {
        toleranceAmountWei: expect.any(BigInt),
        maxValidatorWithdrawalRequestsPerTransaction: expect.any(Number),
        minWithdrawalThresholdEth: expect.any(BigInt),
        stakingRebalanceQuotaBps: expect.any(Number),
        stakingRebalanceQuotaWindowSizeInCycles: expect.any(Number),
      },
      reporting: {
        shouldSubmitVaultReport: expect.any(Boolean),
        shouldReportYield: expect.any(Boolean),
        isUnpauseStakingEnabled: expect.any(Boolean),
        minNegativeYieldDiffToReportYieldWei: expect.any(BigInt),
        cyclesPerYieldReport: expect.any(Number),
      },
      web3signer: {
        url: TEST_WEB3SIGNER_URL,
        publicKey: TEST_WEB3SIGNER_PUBLIC_KEY,
        keystore: {
          path: TEST_WEB3SIGNER_KEYSTORE_PATH,
          passphrase: TEST_WEB3SIGNER_KEYSTORE_PASSPHRASE,
        },
        truststore: {
          path: TEST_WEB3SIGNER_TRUSTSTORE_PATH,
          passphrase: TEST_WEB3SIGNER_TRUSTSTORE_PASSPHRASE,
        },
        tlsEnabled: expect.any(Boolean),
      },
    });
  });

  it("creates logger options with console transport", () => {
    // Arrange
    const env = configSchema.parse(createValidEnv());

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.transports).toHaveLength(1);
    expect(config.loggerOptions.transports[0]).toBeInstanceOf(transports.Console);
  });

  it("defaults to info log level when LOG_LEVEL is not provided", () => {
    // Arrange
    const env = configSchema.parse(createValidEnv());

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(DEFAULT_LOG_LEVEL);
  });

  it("uses custom log level when LOG_LEVEL is provided", () => {
    // Arrange
    const customLogLevel = "debug";
    const env = configSchema.parse(createEnvWithLogLevel(customLogLevel));

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(customLogLevel);
  });

  it("accepts error log level", () => {
    // Arrange
    const logLevel = "error";
    const env = configSchema.parse(createEnvWithLogLevel(logLevel));

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(logLevel);
  });

  it("accepts warn log level", () => {
    // Arrange
    const logLevel = "warn";
    const env = configSchema.parse(createEnvWithLogLevel(logLevel));

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(logLevel);
  });

  it("accepts info log level", () => {
    // Arrange
    const logLevel = "info";
    const env = configSchema.parse(createEnvWithLogLevel(logLevel));

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(logLevel);
  });

  it("accepts verbose log level", () => {
    // Arrange
    const logLevel = "verbose";
    const env = configSchema.parse(createEnvWithLogLevel(logLevel));

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(logLevel);
  });

  it("accepts debug log level", () => {
    // Arrange
    const logLevel = "debug";
    const env = configSchema.parse(createEnvWithLogLevel(logLevel));

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(logLevel);
  });

  it("accepts silly log level", () => {
    // Arrange
    const logLevel = "silly";
    const env = configSchema.parse(createEnvWithLogLevel(logLevel));

    // Act
    const config = toClientConfig(env);

    // Assert
    expect(config.loggerOptions.level).toBe(logLevel);
  });
});
