import { configSchema } from "../config.schema.js";
import { getAddress } from "viem";

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
  WEB3SIGNER_PUBLIC_KEY: `0x${"a".repeat(128)}`,
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

describe("configSchema", () => {
  it("parses and normalizes valid environment variables", () => {
    const env = createValidEnv();

    const parsed = configSchema.parse(env);

    expect(parsed.CHAIN_ID).toBe(11155111);
    expect(parsed.TRIGGER_EVENT_POLL_INTERVAL_MS).toBe(1000);
    expect(parsed.API_PORT).toBe(3000);
    expect(parsed.MIN_WITHDRAWAL_THRESHOLD_ETH).toBe(42n);
    expect(parsed.MIN_POSITIVE_YIELD_TO_REPORT_WEI).toBe(1000000000000000000n);
    expect(parsed.MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI).toBe(500000000000000000n);
    expect(parsed.WEB3SIGNER_TLS_ENABLED).toBe(true);
    expect(parsed.SHOULD_SUBMIT_VAULT_REPORT).toBe(true);
    expect(parsed.LINEA_ROLLUP_ADDRESS).toBe(getAddress(env.LINEA_ROLLUP_ADDRESS));
    expect(parsed.LAZY_ORACLE_ADDRESS).toBe(getAddress(env.LAZY_ORACLE_ADDRESS));
    expect(parsed.VAULT_HUB_ADDRESS).toBe(getAddress(env.VAULT_HUB_ADDRESS));
    expect(parsed.L2_YIELD_RECIPIENT).toBe(getAddress(env.L2_YIELD_RECIPIENT));
  });

  it("rejects invalid Ethereum addresses", () => {
    const env = {
      ...createValidEnv(),
      LINEA_ROLLUP_ADDRESS: "0xinvalid",
    };

    const result = configSchema.safeParse(env);

    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.some((issue) => issue.path.join(".") === "LINEA_ROLLUP_ADDRESS")).toBe(true);
    }
  });

  it("rejects invalid Web3Signer public key values", () => {
    const env = {
      ...createValidEnv(),
      WEB3SIGNER_PUBLIC_KEY: "0x1234",
    };

    const result = configSchema.safeParse(env);

    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.some((issue) => issue.path.join(".") === "WEB3SIGNER_PUBLIC_KEY")).toBe(true);
    }
  });

  describe("MIN_POSITIVE_YIELD_TO_REPORT_WEI", () => {
    it("parses string values to bigint", () => {
      const env = {
        ...createValidEnv(),
        MIN_POSITIVE_YIELD_TO_REPORT_WEI: "1000000000000000000",
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_POSITIVE_YIELD_TO_REPORT_WEI).toBe(1000000000000000000n);
    });

    it("parses number values to bigint", () => {
      const env = {
        ...createValidEnv(),
        MIN_POSITIVE_YIELD_TO_REPORT_WEI: 2000000000000000000,
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_POSITIVE_YIELD_TO_REPORT_WEI).toBe(2000000000000000000n);
    });

    it("parses bigint values", () => {
      const env = {
        ...createValidEnv(),
        MIN_POSITIVE_YIELD_TO_REPORT_WEI: 3000000000000000000n,
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_POSITIVE_YIELD_TO_REPORT_WEI).toBe(3000000000000000000n);
    });

    it("accepts zero value", () => {
      const env = {
        ...createValidEnv(),
        MIN_POSITIVE_YIELD_TO_REPORT_WEI: "0",
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_POSITIVE_YIELD_TO_REPORT_WEI).toBe(0n);
    });

    it("rejects negative values", () => {
      const env = {
        ...createValidEnv(),
        MIN_POSITIVE_YIELD_TO_REPORT_WEI: "-1",
      };

      const result = configSchema.safeParse(env);

      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "MIN_POSITIVE_YIELD_TO_REPORT_WEI")).toBe(true);
      }
    });
  });

  describe("MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI", () => {
    it("parses string values to bigint", () => {
      const env = {
        ...createValidEnv(),
        MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI: "500000000000000000",
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI).toBe(500000000000000000n);
    });

    it("parses number values to bigint", () => {
      const env = {
        ...createValidEnv(),
        MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI: 1000000000000000000,
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI).toBe(1000000000000000000n);
    });

    it("parses bigint values", () => {
      const env = {
        ...createValidEnv(),
        MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI: 1500000000000000000n,
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI).toBe(1500000000000000000n);
    });

    it("accepts zero value", () => {
      const env = {
        ...createValidEnv(),
        MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI: "0",
      };

      const parsed = configSchema.parse(env);

      expect(parsed.MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI).toBe(0n);
    });

    it("rejects negative values", () => {
      const env = {
        ...createValidEnv(),
        MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI: "-1",
      };

      const result = configSchema.safeParse(env);

      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.some((issue) => issue.path.join(".") === "MIN_UNPAID_LIDO_PROTOCOL_FEES_TO_REPORT_YIELD_WEI")).toBe(true);
      }
    });
  });
});
