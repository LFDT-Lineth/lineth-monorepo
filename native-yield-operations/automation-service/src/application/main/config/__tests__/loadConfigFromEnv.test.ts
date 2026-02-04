import { jest, describe, it, expect, beforeEach, afterEach } from "@jest/globals";

import { loadConfigFromEnv } from "../loadConfigFromEnv.js";
import { configSchema } from "../config.schema.js";
import * as configModule from "../config.js";

// Semantic constants
const VALID_CHAIN_ID = "11155111";
const VALID_API_PORT = "3000";
const INVALID_API_PORT = "80"; // below minimum of 1024
const VALID_ADDRESS_1 = "0x1111111111111111111111111111111111111111";
const VALID_ADDRESS_2 = "0x2222222222222222222222222222222222222222";
const VALID_ADDRESS_3 = "0x3333333333333333333333333333333333333333";
const VALID_ADDRESS_4 = "0x4444444444444444444444444444444444444444";
const VALID_ADDRESS_5 = "0x5555555555555555555555555555555555555555";
const VALID_ADDRESS_6 = "0x6666666666666666666666666666666666666666";
const VALID_ADDRESS_7 = "0x7777777777777777777777777777777777777777";
const VALID_BLS_PUBLIC_KEY = `0x${"c".repeat(128)}`;
const PROCESS_EXIT_FAILURE_CODE = 1;

const createValidEnv = (): NodeJS.ProcessEnv => ({
  CHAIN_ID: VALID_CHAIN_ID,
  L1_RPC_URL: "https://rpc.linea.build",
  BEACON_CHAIN_RPC_URL: "https://beacon.linea.build",
  STAKING_GRAPHQL_URL: "https://staking.linea.build/graphql",
  IPFS_BASE_URL: "https://ipfs.linea.build",
  CONSENSYS_STAKING_OAUTH2_TOKEN_ENDPOINT: "https://auth.linea.build/token",
  CONSENSYS_STAKING_OAUTH2_CLIENT_ID: "client-id",
  CONSENSYS_STAKING_OAUTH2_CLIENT_SECRET: "client-secret",
  CONSENSYS_STAKING_OAUTH2_AUDIENCE: "audience",
  LINEA_ROLLUP_ADDRESS: VALID_ADDRESS_1,
  LAZY_ORACLE_ADDRESS: VALID_ADDRESS_2,
  VAULT_HUB_ADDRESS: VALID_ADDRESS_3,
  YIELD_MANAGER_ADDRESS: VALID_ADDRESS_4,
  LIDO_YIELD_PROVIDER_ADDRESS: VALID_ADDRESS_5,
  STETH_ADDRESS: VALID_ADDRESS_6,
  L2_YIELD_RECIPIENT: VALID_ADDRESS_7,
  TRIGGER_EVENT_POLL_INTERVAL_MS: "1000",
  TRIGGER_MAX_INACTION_MS: "5000",
  CONTRACT_READ_RETRY_TIME_MS: "250",
  GAUGE_METRICS_POLL_INTERVAL_MS: "5000",
  REBALANCE_TOLERANCE_AMOUNT_WEI: "5000000000000000000",
  MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION: "16",
  MIN_WITHDRAWAL_THRESHOLD_ETH: "42",
  STAKING_REBALANCE_QUOTA_BPS: "1800",
  STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: "24",
  WEB3SIGNER_URL: "https://web3signer.linea.build",
  WEB3SIGNER_PUBLIC_KEY: VALID_BLS_PUBLIC_KEY,
  WEB3SIGNER_KEYSTORE_PATH: "/path/to/keystore",
  WEB3SIGNER_KEYSTORE_PASSPHRASE: "keystore-pass",
  WEB3SIGNER_TRUSTSTORE_PATH: "/path/to/truststore",
  WEB3SIGNER_TRUSTSTORE_PASSPHRASE: "truststore-pass",
  WEB3SIGNER_TLS_ENABLED: "true",
  API_PORT: VALID_API_PORT,
  SHOULD_SUBMIT_VAULT_REPORT: "true",
  SHOULD_REPORT_YIELD: "true",
  IS_UNPAUSE_STAKING_ENABLED: "true",
  MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: "1000000000000000000",
  CYCLES_PER_YIELD_REPORT: "12",
});

const createMockClientConfig = (sentinel: string): ReturnType<typeof configModule.toClientConfig> =>
  ({ sentinel }) as unknown as ReturnType<typeof configModule.toClientConfig>;

describe("loadConfigFromEnv", () => {
  let originalEnv: NodeJS.ProcessEnv;
  let consoleErrorSpy: jest.SpiedFunction<typeof console.error>;
  let processExitSpy: jest.SpiedFunction<typeof process.exit>;
  let toClientConfigSpy: jest.SpiedFunction<typeof configModule.toClientConfig>;

  beforeEach(() => {
    originalEnv = process.env;
  });

  afterEach(() => {
    process.env = originalEnv;
    jest.restoreAllMocks();
  });

  it("parses valid environment and returns client config", () => {
    // Arrange
    const env = createValidEnv();
    const expectedConfig = createMockClientConfig("value");
    toClientConfigSpy = jest.spyOn(configModule, "toClientConfig").mockReturnValue(expectedConfig);

    // Act
    const result = loadConfigFromEnv(env);

    // Assert
    expect(result).toBe(expectedConfig);
    expect(toClientConfigSpy).toHaveBeenCalledTimes(1);
    expect(toClientConfigSpy).toHaveBeenCalledWith(configSchema.parse(env));
  });

  it("uses process.env when no environment object provided", () => {
    // Arrange
    const env = createValidEnv();
    const expectedConfig = createMockClientConfig("process-env");
    toClientConfigSpy = jest.spyOn(configModule, "toClientConfig").mockReturnValue(expectedConfig);
    process.env = { ...env } as unknown as NodeJS.ProcessEnv;

    // Act
    const result = loadConfigFromEnv();

    // Assert
    expect(result).toBe(expectedConfig);
    expect(toClientConfigSpy).toHaveBeenCalledWith(configSchema.parse(env));
  });

  it("logs error and exits process when validation fails", () => {
    // Arrange
    const env = {
      ...createValidEnv(),
      API_PORT: INVALID_API_PORT,
    };
    consoleErrorSpy = jest.spyOn(console, "error").mockImplementation(() => {});
    processExitSpy = jest.spyOn(process, "exit").mockImplementation(((code?: number | undefined) => {
      throw new Error(`process.exit: ${code}`);
    }) as never);
    toClientConfigSpy = jest.spyOn(configModule, "toClientConfig");

    // Act & Assert
    expect(() => loadConfigFromEnv(env)).toThrow(`process.exit: ${PROCESS_EXIT_FAILURE_CODE}`);
    expect(consoleErrorSpy).toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("Invalid configuration"));
    expect(processExitSpy).toHaveBeenCalledWith(PROCESS_EXIT_FAILURE_CODE);
    expect(toClientConfigSpy).not.toHaveBeenCalled();
  });
});
