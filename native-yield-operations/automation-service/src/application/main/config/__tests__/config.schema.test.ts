import { configSchema } from "../config.schema.js";
import { getAddress } from "viem";

const VALID_CHAIN_ID = "11155111";
const INVALID_ETH_ADDRESS = "0xinvalid";
const VALID_WEB3SIGNER_PUBLIC_KEY = `0x${"a".repeat(128)}`;
const INVALID_WEB3SIGNER_PUBLIC_KEY = "0x1234";
const VALID_BIGINT_STRING = "1000000000000000000";
const VALID_BPS = "1800";
const VALID_CYCLES = "24";
const VALID_WITHDRAWAL_THRESHOLD = "42";

const createValidEnv = () => ({
  CHAIN_ID: VALID_CHAIN_ID,
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
  STETH_ADDRESS: "0x6666666666666666666666666666666666666666",
  L2_YIELD_RECIPIENT: "0x7777777777777777777777777777777777777777",
  TRIGGER_EVENT_POLL_INTERVAL_MS: "1000",
  TRIGGER_MAX_INACTION_MS: "5000",
  CONTRACT_READ_RETRY_TIME_MS: "250",
  GAUGE_METRICS_POLL_INTERVAL_MS: "5000",
  REBALANCE_TOLERANCE_AMOUNT_WEI: "5000000000000000000",
  MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION: "16",
  MIN_WITHDRAWAL_THRESHOLD_ETH: VALID_WITHDRAWAL_THRESHOLD,
  STAKING_REBALANCE_QUOTA_BPS: VALID_BPS,
  STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: VALID_CYCLES,
  WEB3SIGNER_URL: "https://web3signer.linea.build",
  WEB3SIGNER_PUBLIC_KEY: VALID_WEB3SIGNER_PUBLIC_KEY,
  WEB3SIGNER_KEYSTORE_PATH: "/path/to/keystore",
  WEB3SIGNER_KEYSTORE_PASSPHRASE: "keystore-pass",
  WEB3SIGNER_TRUSTSTORE_PATH: "/path/to/truststore",
  WEB3SIGNER_TRUSTSTORE_PASSPHRASE: "truststore-pass",
  WEB3SIGNER_TLS_ENABLED: "true",
  API_PORT: "3000",
  SHOULD_SUBMIT_VAULT_REPORT: "true",
  SHOULD_REPORT_YIELD: "true",
  IS_UNPAUSE_STAKING_ENABLED: "true",
  MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: VALID_BIGINT_STRING,
  CYCLES_PER_YIELD_REPORT: "12",
});

const createEnvWithInvalidAddress = (addressField: string) => ({
  ...createValidEnv(),
  [addressField]: INVALID_ETH_ADDRESS,
});

const createEnvWithInvalidWeb3SignerKey = () => ({
  ...createValidEnv(),
  WEB3SIGNER_PUBLIC_KEY: INVALID_WEB3SIGNER_PUBLIC_KEY,
});

describe("configSchema", () => {
  it("parses and normalizes valid environment variables", () => {
    // Arrange
    const env = createValidEnv();

    // Act
    const parsed = configSchema.parse(env);

    // Assert
    expect(parsed.CHAIN_ID).toBe(11155111);
    expect(parsed.TRIGGER_EVENT_POLL_INTERVAL_MS).toBe(1000);
    expect(parsed.API_PORT).toBe(3000);
    expect(parsed.MIN_WITHDRAWAL_THRESHOLD_ETH).toBe(42n);
    expect(parsed.STAKING_REBALANCE_QUOTA_BPS).toBe(1800);
    expect(parsed.STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES).toBe(24);
    expect(parsed.MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI).toBe(1000000000000000000n);
    expect(parsed.WEB3SIGNER_TLS_ENABLED).toBe(true);
    expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(true);
    expect(parsed.SHOULD_REPORT_YIELD).toBe(true);
    expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(true);
    expect(parsed.LINEA_ROLLUP_ADDRESS).toBe(getAddress(env.LINEA_ROLLUP_ADDRESS));
    expect(parsed.LAZY_ORACLE_ADDRESS).toBe(getAddress(env.LAZY_ORACLE_ADDRESS));
    expect(parsed.VAULT_HUB_ADDRESS).toBe(getAddress(env.VAULT_HUB_ADDRESS));
    expect(parsed.STETH_ADDRESS).toBe(getAddress(env.STETH_ADDRESS));
    expect(parsed.L2_YIELD_RECIPIENT).toBe(getAddress(env.L2_YIELD_RECIPIENT));
  });

  it("rejects invalid Ethereum addresses", () => {
    // Arrange
    const env = createEnvWithInvalidAddress("LINEA_ROLLUP_ADDRESS");

    // Act
    const result = configSchema.safeParse(env);

    // Assert
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.some((issue) => issue.path.join(".") === "LINEA_ROLLUP_ADDRESS")).toBe(true);
    }
  });

  it("rejects invalid Web3Signer public key values", () => {
    // Arrange
    const env = createEnvWithInvalidWeb3SignerKey();

    // Act
    const result = configSchema.safeParse(env);

    // Assert
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.some((issue) => issue.path.join(".") === "WEB3SIGNER_PUBLIC_KEY")).toBe(true);
    }
  });

  describe("MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI", () => {
    it("parses string values to bigint", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: "1000000000000000000",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI).toBe(1000000000000000000n);
    });

    it("parses number values to bigint", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: 2000000000000000000,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI).toBe(2000000000000000000n);
    });

    it("parses bigint values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: 3000000000000000000n,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI).toBe(3000000000000000000n);
    });

    it("accepts zero value", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: "0",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI).toBe(0n);
    });

    it("rejects negative values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI: "-1",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(
          result.error.issues.some((issue) => issue.path.join(".") === "MIN_NEGATIVE_YIELD_DIFF_TO_REPORT_YIELD_WEI"),
        ).toBe(true);
      }
    });
  });

  describe("STAKING_REBALANCE_QUOTA_BPS", () => {
    it("parses string values to number", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_BPS: "1800",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.STAKING_REBALANCE_QUOTA_BPS).toBe(1800);
    });

    it("parses number values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_BPS: 2000,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.STAKING_REBALANCE_QUOTA_BPS).toBe(2000);
    });

    it("accepts zero value (disables quota mechanism)", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_BPS: "0",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.STAKING_REBALANCE_QUOTA_BPS).toBe(0);
    });

    it("rejects negative values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_BPS: "-1",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "STAKING_REBALANCE_QUOTA_BPS")).toBe(true);
      }
    });

    it("rejects non-integer values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_BPS: "1800.5",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "STAKING_REBALANCE_QUOTA_BPS")).toBe(true);
      }
    });
  });

  describe("STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES", () => {
    it("parses string values to number", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: "24",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES).toBe(24);
    });

    it("parses number values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: 48,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES).toBe(48);
    });

    it("accepts zero value (disables quota mechanism)", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: "0",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES).toBe(0);
    });

    it("rejects negative values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: "-1",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(
          result.error.issues.some((issue) => issue.path.join(".") === "STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES"),
        ).toBe(true);
      }
    });

    it("rejects non-integer values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES: "24.5",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(
          result.error.issues.some((issue) => issue.path.join(".") === "STAKING_REBALANCE_QUOTA_WINDOW_SIZE_IN_CYCLES"),
        ).toBe(true);
      }
    });
  });

  describe("SHOULD_SUBMIT_VAULT_REPORT", () => {
    it("parses string 'false' as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "false",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(false);
    });

    it("parses string 'true' as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "true",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(true);
    });

    it("parses case-insensitive string 'FALSE' as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "FALSE",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(false);
    });

    it("parses case-insensitive string 'True' as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "True",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(true);
    });

    it("parses numeric string '0' as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "0",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(false);
    });

    it("parses numeric string '1' as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "1",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(true);
    });

    it("parses actual boolean false as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: false,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(false);
    });

    it("parses actual boolean true as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: true,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(true);
    });

    it("parses numeric 0 as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: 0,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(false);
    });

    it("parses numeric 1 as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: 1,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(true);
    });

    it("handles strings with whitespace", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "  false  ",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(false);
    });

    it("rejects invalid string values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "invalid",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "SHOULD_SUBMIT_VAULT_REPORT")).toBe(true);
      }
    });

    it("rejects empty string", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        SHOULD_SUBMIT_VAULT_REPORT: "",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "SHOULD_SUBMIT_VAULT_REPORT")).toBe(true);
      }
    });
  });

  describe("IS_UNPAUSE_STAKING_ENABLED", () => {
    it("parses string 'false' as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "false",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(false);
    });

    it("parses string 'true' as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "true",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(true);
    });

    it("parses case-insensitive string 'FALSE' as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "FALSE",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(false);
    });

    it("parses case-insensitive string 'True' as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "True",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(true);
    });

    it("parses numeric string '0' as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "0",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(false);
    });

    it("parses numeric string '1' as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "1",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(true);
    });

    it("parses actual boolean false as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: false,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(false);
    });

    it("parses actual boolean true as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: true,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(true);
    });

    it("parses numeric 0 as false", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: 0,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(false);
    });

    it("parses numeric 1 as true", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: 1,
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(true);
    });

    it("handles strings with whitespace", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "  false  ",
      };

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.IS_UNPAUSE_STAKING_ENABLED).toBe(false);
    });

    it("rejects invalid string values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "invalid",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "IS_UNPAUSE_STAKING_ENABLED")).toBe(true);
      }
    });

    it("rejects empty string", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        IS_UNPAUSE_STAKING_ENABLED: "",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "IS_UNPAUSE_STAKING_ENABLED")).toBe(true);
      }
    });
  });

  describe("MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION", () => {
    it("accepts zero value (disables withdrawal requests)", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION: "0",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION).toBe(0);
      }
    });

    it("rejects negative values", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION: "-1",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(
          result.error.issues.some(
            (issue) => issue.path.join(".") === "MAX_VALIDATOR_WITHDRAWAL_REQUESTS_PER_TRANSACTION",
          ),
        ).toBe(true);
      }
    });
  });

  describe("LOG_LEVEL", () => {
    it("accepts valid log levels", () => {
      // Arrange
      const validLevels = ["error", "warn", "info", "verbose", "debug", "silly"];

      for (const level of validLevels) {
        const env = {
          ...createValidEnv(),
          LOG_LEVEL: level,
        };

        // Act
        const parsed = configSchema.parse(env);

        // Assert
        expect(parsed.LOG_LEVEL).toBe(level);
      }
    });

    it("accepts undefined LOG_LEVEL (optional)", () => {
      // Arrange
      const env = createValidEnv();

      // Act
      const parsed = configSchema.parse(env);

      // Assert
      expect(parsed.LOG_LEVEL).toBeUndefined();
    });

    it("rejects invalid log levels", () => {
      // Arrange
      const env = {
        ...createValidEnv(),
        LOG_LEVEL: "invalid",
      };

      // Act
      const result = configSchema.safeParse(env);

      // Assert
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "LOG_LEVEL")).toBe(true);
      }
    });
  });
});
