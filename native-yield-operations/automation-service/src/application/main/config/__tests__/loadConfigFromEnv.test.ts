import { jest } from "@jest/globals";
import { loadConfigFromEnv } from "../loadConfigFromEnv.js";
import { configSchema } from "../config.schema.js";
import * as configModule from "../config.js";

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
  WEB3SIGNER_PUBLIC_KEY: `0x${"c".repeat(128)}`,
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

describe("loadConfigFromEnv", () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("returns the bootstrap config when the environment is valid", () => {
    const env = createValidEnv();
    const expectedConfig = { sentinel: "value" } as unknown as ReturnType<typeof configModule.toClientConfig>;
    const toClientConfigSpy = jest.spyOn(configModule, "toClientConfig").mockReturnValue(expectedConfig);

    const result = loadConfigFromEnv(env);

    expect(result).toBe(expectedConfig);
    expect(toClientConfigSpy).toHaveBeenCalledTimes(1);
    expect(toClientConfigSpy).toHaveBeenCalledWith(configSchema.parse(env));
  });

  it("falls back to process.env when no environment object is provided", () => {
    const env = createValidEnv();
    const expectedConfig = { sentinel: "process-env" } as unknown as ReturnType<typeof configModule.toClientConfig>;
    const toClientConfigSpy = jest.spyOn(configModule, "toClientConfig").mockReturnValue(expectedConfig);
    const originalEnv = process.env;
    process.env = { ...env } as unknown as NodeJS.ProcessEnv;

    try {
      const result = loadConfigFromEnv();

      expect(result).toBe(expectedConfig);
      expect(toClientConfigSpy).toHaveBeenCalledWith(configSchema.parse(env));
    } finally {
      process.env = originalEnv;
    }
  });

  it("logs errors and exits the process when validation fails", () => {
    const env = {
      ...createValidEnv(),
      API_PORT: "80", // below minimum of 1024
    };
    const consoleSpy = jest.spyOn(console, "error").mockImplementation(() => {});
    const exitSpy = jest.spyOn(process, "exit").mockImplementation(((code?: number | undefined) => {
      throw new Error(`process.exit: ${code}`);
    }) as never);
    const toClientConfigSpy = jest.spyOn(configModule, "toClientConfig");

    expect(() => loadConfigFromEnv(env)).toThrow("process.exit: 1");
    expect(consoleSpy).toHaveBeenCalled();
    expect(consoleSpy).toHaveBeenCalledWith(expect.stringContaining("Invalid configuration"));
    expect(exitSpy).toHaveBeenCalledWith(1);
    expect(toClientConfigSpy).not.toHaveBeenCalled();
  });
});
