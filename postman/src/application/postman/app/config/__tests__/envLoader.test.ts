import { describe, it, expect } from "@jest/globals";

import {
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_L1_SIGNER_PRIVATE_KEY,
  TEST_L2_SIGNER_PRIVATE_KEY,
} from "../../../../../utils/testing/constants";
import { TEST_ENV_VARS } from "../../../../../utils/testing/fixtures";
import { withEnv } from "../../../../../utils/testing/helpers";
import { loadPostmanOptionsFromEnv } from "../envLoader";

describe("loadPostmanOptionsFromEnv", () => {
  it("should load minimal config from environment variables", async () => {
    await withEnv(TEST_ENV_VARS, () => {
      const options = loadPostmanOptionsFromEnv();

      expect(options.l1Options.rpcUrl).toBe("http://localhost:8445");
      expect(options.l2Options.rpcUrl).toBe("http://localhost:8545");
      expect(options.l1Options.messageServiceContractAddress).toBe(TEST_CONTRACT_ADDRESS_1);
      expect(options.l2Options.messageServiceContractAddress).toBe(TEST_CONTRACT_ADDRESS_2);
      expect(options.l1L2AutoClaimEnabled).toBe(true);
      expect(options.l2L1AutoClaimEnabled).toBe(true);
    });
  });

  it("should build private-key signer config by default", async () => {
    await withEnv(TEST_ENV_VARS, () => {
      const options = loadPostmanOptionsFromEnv();

      expect(options.l1Options.claiming.signer).toEqual({
        type: "private-key",
        privateKey: TEST_L1_SIGNER_PRIVATE_KEY,
      });
      expect(options.l2Options.claiming.signer).toEqual({
        type: "private-key",
        privateKey: TEST_L2_SIGNER_PRIVATE_KEY,
      });
    });
  });

  it("should prefix L1/L2 signer private keys with 0x when missing", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_SIGNER_PRIVATE_KEY: TEST_L1_SIGNER_PRIVATE_KEY.slice(2),
        L2_SIGNER_PRIVATE_KEY: TEST_L2_SIGNER_PRIVATE_KEY.slice(2),
      },
      () => {
        const options = loadPostmanOptionsFromEnv();
        expect(options.l1Options.claiming.signer).toEqual({
          type: "private-key",
          privateKey: TEST_L1_SIGNER_PRIVATE_KEY,
        });
        expect(options.l2Options.claiming.signer).toEqual({
          type: "private-key",
          privateKey: TEST_L2_SIGNER_PRIVATE_KEY,
        });
      },
    );
  });

  it("should build web3signer config when L1_SIGNER_TYPE is web3signer", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_SIGNER_TYPE: "web3signer",
        L1_WEB3_SIGNER_ENDPOINT: "https://signer.example.com",
        L1_WEB3_SIGNER_PUBLIC_KEY: "0xaabb",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.claiming.signer).toEqual({
          type: "web3signer",
          endpoint: "https://signer.example.com",
          publicKey: "0xaabb",
        });
      },
    );
  });

  it("should include TLS config for web3signer when TLS env vars are set", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_SIGNER_TYPE: "web3signer",
        L1_WEB3_SIGNER_ENDPOINT: "https://signer.example.com",
        L1_WEB3_SIGNER_PUBLIC_KEY: "0xaabb",
        L1_WEB3_SIGNER_TLS_KEYSTORE_PATH: "/path/to/keystore",
        L1_WEB3_SIGNER_TLS_KEYSTORE_PASSWORD: "keypass",
        L1_WEB3_SIGNER_TLS_TRUSTSTORE_PATH: "/path/to/truststore",
        L1_WEB3_SIGNER_TLS_TRUSTSTORE_PASSWORD: "trustpass",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.claiming.signer).toEqual({
          type: "web3signer",
          endpoint: "https://signer.example.com",
          publicKey: "0xaabb",
          tls: {
            keyStorePath: "/path/to/keystore",
            keyStorePassword: "keypass",
            trustStorePath: "/path/to/truststore",
            trustStorePassword: "trustpass",
          },
        });
      },
    );
  });

  it("should parse listener options from env vars", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_LISTENER_INTERVAL: "5000",
        L1_LISTENER_RECEIPT_POLLING_INTERVAL: "3000",
        MAX_FETCH_MESSAGES_FROM_DB: "10",
        L1_MAX_BLOCKS_TO_FETCH_LOGS: "500",
        L1_LISTENER_INITIAL_FROM_BLOCK: "100",
        L1_LISTENER_BLOCK_CONFIRMATION: "12",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.listener.pollingInterval).toBe(5000);
        expect(options.l1Options.listener.receiptPollingInterval).toBe(3000);
        expect(options.l1Options.listener.maxFetchMessagesFromDb).toBe(10);
        expect(options.l1Options.listener.maxBlocksToFetchLogs).toBe(500);
        expect(options.l1Options.listener.initialFromBlock).toBe(100);
        expect(options.l1Options.listener.blockConfirmation).toBe(12);
      },
    );
  });

  it("should parse claiming options from env vars", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        MESSAGE_SUBMISSION_TIMEOUT: "600000",
        MAX_NONCE_DIFF: "20",
        MAX_FEE_PER_GAS_CAP: "200000000000",
        GAS_ESTIMATION_PERCENTILE: "25",
        PROFIT_MARGIN: "1.5",
        MAX_NUMBER_OF_RETRIES: "5",
        RETRY_DELAY_IN_SECONDS: "30",
        MAX_CLAIM_GAS_LIMIT: "100000",
        MAX_BUMPS_PER_CYCLE: "3",
        MAX_RETRY_CYCLES: "10",
        L1_MAX_GAS_FEE_ENFORCED: "true",
        L2_L1_ENABLE_POSTMAN_SPONSORING: "true",
        MAX_POSTMAN_SPONSOR_GAS_LIMIT: "50000",
        L1_CLAIM_VIA_ADDRESS: TEST_CONTRACT_ADDRESS_1,
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.claiming.messageSubmissionTimeout).toBe(600000);
        expect(options.l1Options.claiming.maxNonceDiff).toBe(20);
        expect(options.l1Options.claiming.maxFeePerGasCap).toBe(200000000000n);
        expect(options.l1Options.claiming.gasEstimationPercentile).toBe(25);
        expect(options.l1Options.claiming.profitMargin).toBe(1.5);
        expect(options.l1Options.claiming.maxNumberOfRetries).toBe(5);
        expect(options.l1Options.claiming.retryDelayInSeconds).toBe(30);
        expect(options.l1Options.claiming.maxClaimGasLimit).toBe(100000n);
        expect(options.l1Options.claiming.maxBumpsPerCycle).toBe(3);
        expect(options.l1Options.claiming.maxRetryCycles).toBe(10);
        expect(options.l1Options.claiming.isMaxGasFeeEnforced).toBe(true);
        expect(options.l1Options.claiming.isPostmanSponsorshipEnabled).toBe(true);
        expect(options.l1Options.claiming.maxPostmanSponsorGasLimit).toBe(50000n);
        expect(options.l1Options.claiming.claimViaAddress).toBe(TEST_CONTRACT_ADDRESS_1);
      },
    );
  });

  it("should parse event filter env vars", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_EVENT_FILTER_FROM_ADDRESS: TEST_CONTRACT_ADDRESS_1,
        L1_EVENT_FILTER_TO_ADDRESS: TEST_CONTRACT_ADDRESS_2,
        L1_EVENT_FILTER_CALLDATA: "calldata.amount > 0",
        L1_EVENT_FILTER_CALLDATA_FUNCTION_INTERFACE: "function transfer(address to, uint256 amount)",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.listener.eventFilters).toEqual({
          fromAddressFilter: TEST_CONTRACT_ADDRESS_1,
          toAddressFilter: TEST_CONTRACT_ADDRESS_2,
          calldataFilter: {
            criteriaExpression: "calldata.amount > 0",
            calldataFunctionInterface: "function transfer(address to, uint256 amount)",
          },
        });
      },
    );
  });

  it("should not include event filters when no filter env vars are set", async () => {
    await withEnv(TEST_ENV_VARS, () => {
      const options = loadPostmanOptionsFromEnv();
      expect(options.l1Options.listener.eventFilters).toBeUndefined();
    });
  });

  it("should parse database options from env vars", async () => {
    await withEnv(TEST_ENV_VARS, () => {
      const options = loadPostmanOptionsFromEnv();

      expect(options.databaseOptions).toEqual(
        expect.objectContaining({
          type: "postgres",
          host: "localhost",
          port: 5432,
          username: "test_user",
          password: "test_password",
          database: "test_db",
        }),
      );
    });
  });

  it("should parse SSL database options", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        POSTGRES_SSL: "true",
        POSTGRES_SSL_REJECT_UNAUTHORIZED: "true",
        POSTGRES_SSL_CA_PATH: "/path/to/ca.pem",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.databaseOptions).toEqual(
          expect.objectContaining({
            ssl: {
              rejectUnauthorized: true,
              ca: "/path/to/ca.pem",
            },
          }),
        );
      },
    );
  });

  it("should parse database cleaner options", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        DB_CLEANER_ENABLED: "true",
        DB_CLEANING_INTERVAL: "86400000",
        DB_DAYS_BEFORE_NOW_TO_DELETE: "7",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.databaseCleanerOptions).toEqual({
          enabled: true,
          cleaningInterval: 86400000,
          daysBeforeNowToDelete: 7,
        });
      },
    );
  });

  it("should parse API options from env vars", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        API_PORT: "8080",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();
        expect(options.apiOptions).toEqual({ port: 8080 });
      },
    );
  });

  it("should use default values when optional env vars are not set", async () => {
    await withEnv(
      {
        L1_RPC_URL: "http://localhost:8445",
        L2_RPC_URL: "http://localhost:8545",
        L1_CONTRACT_ADDRESS: TEST_CONTRACT_ADDRESS_1,
        L2_CONTRACT_ADDRESS: TEST_CONTRACT_ADDRESS_2,
        L1_SIGNER_PRIVATE_KEY: TEST_L1_SIGNER_PRIVATE_KEY,
        L2_SIGNER_PRIVATE_KEY: TEST_L2_SIGNER_PRIVATE_KEY,
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1L2AutoClaimEnabled).toBe(false);
        expect(options.l2L1AutoClaimEnabled).toBe(false);
        expect(options.l1Options.isEOAEnabled).toBe(false);
        expect(options.l1Options.isCalldataEnabled).toBe(false);
        expect(options.l1Options.claiming.isMaxGasFeeEnforced).toBe(false);
        expect(options.databaseOptions.host).toBe("127.0.0.1");
        expect(options.databaseOptions.port).toBe(5432);
      },
    );
  });

  it("should parse L2 specific options", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L2_MESSAGE_TREE_DEPTH: "32",
        ENABLE_LINEA_ESTIMATE_GAS: "true",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l2Options.l2MessageTreeDepth).toBe(32);
        expect(options.l2Options.enableLineaEstimateGas).toBe(true);
      },
    );
  });

  it("should parse calldata enabled flags", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_L2_CALLDATA_ENABLED: "true",
        L2_L1_CALLDATA_ENABLED: "true",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.isCalldataEnabled).toBe(true);
        expect(options.l2Options.isCalldataEnabled).toBe(true);
      },
    );
  });

  it("should build aws-kms signer config when L1_SIGNER_TYPE is aws-kms", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_SIGNER_TYPE: "aws-kms",
        L1_AWS_KMS_KEY_ID: "alias/linea-postman-l1",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.claiming.signer).toEqual({
          type: "aws-kms",
          kmsKeyId: "alias/linea-postman-l1",
        });
      },
    );
  });

  it("should include AWS region for aws-kms signer when L1_AWS_KMS_REGION is set", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_SIGNER_TYPE: "aws-kms",
        L1_AWS_KMS_KEY_ID: "arn:aws:kms:eu-west-1:000000000000:key/abcd-1234",
        L1_AWS_KMS_REGION: "eu-west-1",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.claiming.signer).toEqual({
          type: "aws-kms",
          kmsKeyId: "arn:aws:kms:eu-west-1:000000000000:key/abcd-1234",
          region: "eu-west-1",
        });
      },
    );
  });

  it("should build aws-kms signer config for L2 when L2_SIGNER_TYPE is aws-kms", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L2_SIGNER_TYPE: "aws-kms",
        L2_AWS_KMS_KEY_ID: "alias/linea-postman-l2",
        L2_AWS_KMS_REGION: "us-east-1",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l2Options.claiming.signer).toEqual({
          type: "aws-kms",
          kmsKeyId: "alias/linea-postman-l2",
          region: "us-east-1",
        });
      },
    );
  });

  it("should build web3signer config for L2 when L2_SIGNER_TYPE is web3signer", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L2_SIGNER_TYPE: "web3signer",
        L2_WEB3_SIGNER_ENDPOINT: "https://l2-signer.example.com",
        L2_WEB3_SIGNER_PUBLIC_KEY: "0xccdd",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l2Options.claiming.signer).toEqual({
          type: "web3signer",
          endpoint: "https://l2-signer.example.com",
          publicKey: "0xccdd",
        });
      },
    );
  });

  it("should build event filters with only fromAddress", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_EVENT_FILTER_FROM_ADDRESS: TEST_CONTRACT_ADDRESS_1,
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.listener.eventFilters).toEqual({
          fromAddressFilter: TEST_CONTRACT_ADDRESS_1,
          toAddressFilter: undefined,
        });
      },
    );
  });

  it("should build event filters with only toAddress", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L2_EVENT_FILTER_TO_ADDRESS: TEST_CONTRACT_ADDRESS_2,
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l2Options.listener.eventFilters).toEqual({
          fromAddressFilter: undefined,
          toAddressFilter: TEST_CONTRACT_ADDRESS_2,
        });
      },
    );
  });

  it("should handle initialFromBlock of 0 as a valid value", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_LISTENER_INITIAL_FROM_BLOCK: "0",
        L1_LISTENER_BLOCK_CONFIRMATION: "0",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.listener.initialFromBlock).toBe(0);
        expect(options.l1Options.listener.blockConfirmation).toBe(0);
      },
    );
  });

  it("should parse L2 listener options", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L2_LISTENER_INTERVAL: "8000",
        L2_LISTENER_INITIAL_FROM_BLOCK: "50",
        L2_LISTENER_BLOCK_CONFIRMATION: "6",
        L2_MAX_BLOCKS_TO_FETCH_LOGS: "200",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l2Options.listener.pollingInterval).toBe(8000);
        expect(options.l2Options.listener.initialFromBlock).toBe(50);
        expect(options.l2Options.listener.blockConfirmation).toBe(6);
        expect(options.l2Options.listener.maxBlocksToFetchLogs).toBe(200);
      },
    );
  });

  it("should parse L2 claiming sponsorship from L1 env var", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_L2_ENABLE_POSTMAN_SPONSORING: "true",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l2Options.claiming.isPostmanSponsorshipEnabled).toBe(true);
      },
    );
  });

  it("should parse SSL options with rejectUnauthorized false when env var is not 'true'", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        POSTGRES_SSL: "true",
        POSTGRES_SSL_REJECT_UNAUTHORIZED: "false",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.databaseOptions.ssl).toEqual({
          rejectUnauthorized: false,
          ca: undefined,
        });
      },
    );
  });

  it("should use defaults for web3signer when endpoint and publicKey env vars are not set", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_SIGNER_TYPE: "web3signer",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.claiming.signer).toEqual({
          type: "web3signer",
          endpoint: "",
          publicKey: "0x",
        });
      },
    );
  });

  it("should use defaults for TLS passwords when only keystore path is set", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_SIGNER_TYPE: "web3signer",
        L1_WEB3_SIGNER_TLS_KEYSTORE_PATH: "/path/to/keystore",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.claiming.signer).toEqual({
          type: "web3signer",
          endpoint: "",
          publicKey: "0x",
          tls: {
            keyStorePath: "/path/to/keystore",
            keyStorePassword: "",
            trustStorePath: "",
            trustStorePassword: "",
          },
        });
      },
    );
  });

  it("should not include calldata filter when only calldataExpr is set without calldataIface", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_EVENT_FILTER_CALLDATA: "calldata.amount > 0",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.listener.eventFilters).toBeUndefined();
      },
    );
  });

  it("should not include calldata filter when only calldataIface is set without calldataExpr", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_EVENT_FILTER_CALLDATA_FUNCTION_INTERFACE: "function transfer(address to, uint256 amount)",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.listener.eventFilters).toBeUndefined();
      },
    );
  });

  it("should include event filters with from address and calldata filter together", async () => {
    await withEnv(
      {
        ...TEST_ENV_VARS,
        L1_EVENT_FILTER_FROM_ADDRESS: TEST_CONTRACT_ADDRESS_1,
        L1_EVENT_FILTER_CALLDATA: "calldata.amount > 0",
        L1_EVENT_FILTER_CALLDATA_FUNCTION_INTERFACE: "function transfer(address to, uint256 amount)",
      },
      () => {
        const options = loadPostmanOptionsFromEnv();

        expect(options.l1Options.listener.eventFilters).toEqual({
          fromAddressFilter: TEST_CONTRACT_ADDRESS_1,
          toAddressFilter: undefined,
          calldataFilter: {
            criteriaExpression: "calldata.amount > 0",
            calldataFunctionInterface: "function transfer(address to, uint256 amount)",
          },
        });
      },
    );
  });

  it("should use empty defaults when no env vars are set at all", async () => {
    await withEnv({}, () => {
      const options = loadPostmanOptionsFromEnv();

      expect(options.l1Options.rpcUrl).toBe("");
      expect(options.l1Options.messageServiceContractAddress).toBe("");
      expect(options.l2Options.rpcUrl).toBe("");
      expect(options.l2Options.messageServiceContractAddress).toBe("");
      expect(options.l1Options.claiming.signer).toEqual({
        type: "private-key",
        privateKey: "0x",
      });
      expect(options.l2Options.claiming.signer).toEqual({
        type: "private-key",
        privateKey: "0x",
      });
    });
  });
});
