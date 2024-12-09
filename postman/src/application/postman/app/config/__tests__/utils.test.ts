import { describe } from "@jest/globals";
import { getConfig } from "../utils";
import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_L1_SIGNER_PRIVATE_KEY,
  TEST_L2_SIGNER_PRIVATE_KEY,
  TEST_RPC_URL,
} from "../../../../../utils/testing/constants";
import {
  DEFAULT_CALLDATA_ENABLED,
  DEFAULT_DB_CLEANER_ENABLED,
  DEFAULT_DB_CLEANING_INTERVAL,
  DEFAULT_DB_DAYS_BEFORE_NOW_TO_DELETE,
  DEFAULT_ENFORCE_MAX_GAS_FEE,
  DEFAULT_EOA_ENABLED,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_INITIAL_FROM_BLOCK,
  DEFAULT_L2_MESSAGE_TREE_DEPTH,
  DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
  DEFAULT_LISTENER_INTERVAL,
  DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
  DEFAULT_MAX_NONCE_DIFF,
  DEFAULT_MAX_NUMBER_OF_RETRIES,
  DEFAULT_MAX_TX_RETRIES,
  DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
  DEFAULT_PROFIT_MARGIN,
  DEFAULT_RETRY_DELAY_IN_SECONDS,
} from "../../../../../core/constants";

describe("Config utils", () => {
  describe("getConfig", () => {
    it("should return the default config when no optional parameters are passed.", () => {
      const config = getConfig({
        l1Options: {
          rpcUrl: TEST_RPC_URL,
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
          listener: {},
          claiming: {
            signerPrivateKey: TEST_L1_SIGNER_PRIVATE_KEY,
          },
        },
        l2Options: {
          rpcUrl: TEST_RPC_URL,
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
          listener: {},
          claiming: {
            signerPrivateKey: TEST_L2_SIGNER_PRIVATE_KEY,
          },
        },
        l1L2AutoClaimEnabled: false,
        l2L1AutoClaimEnabled: false,
        databaseOptions: {
          type: "postgres",
        },
      });
      expect(config).toStrictEqual({
        databaseCleanerConfig: {
          cleaningInterval: DEFAULT_DB_CLEANING_INTERVAL,
          daysBeforeNowToDelete: DEFAULT_DB_DAYS_BEFORE_NOW_TO_DELETE,
          enabled: DEFAULT_DB_CLEANER_ENABLED,
        },
        databaseOptions: {
          type: "postgres",
        },
        l1Config: {
          claiming: {
            feeRecipientAddress: undefined,
            gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
            isMaxGasFeeEnforced: DEFAULT_ENFORCE_MAX_GAS_FEE,
            maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
            maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
            maxNonceDiff: DEFAULT_MAX_NONCE_DIFF,
            maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
            maxTxRetries: DEFAULT_MAX_TX_RETRIES,
            messageSubmissionTimeout: DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
            profitMargin: DEFAULT_PROFIT_MARGIN,
            retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
            signerPrivateKey: TEST_L1_SIGNER_PRIVATE_KEY,
          },
          isCalldataEnabled: DEFAULT_CALLDATA_ENABLED,
          isEOAEnabled: DEFAULT_EOA_ENABLED,
          listener: {
            blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
            initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
            maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
            maxFetchMessagesFromDb: DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
            pollingInterval: DEFAULT_LISTENER_INTERVAL,
          },
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
          rpcUrl: TEST_RPC_URL,
        },
        l1L2AutoClaimEnabled: false,
        l2Config: {
          claiming: {
            feeRecipientAddress: undefined,
            gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
            isMaxGasFeeEnforced: DEFAULT_ENFORCE_MAX_GAS_FEE,
            maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
            maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
            maxNonceDiff: DEFAULT_MAX_NONCE_DIFF,
            maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
            maxTxRetries: DEFAULT_MAX_TX_RETRIES,
            messageSubmissionTimeout: DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
            profitMargin: DEFAULT_PROFIT_MARGIN,
            retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
            signerPrivateKey: TEST_L2_SIGNER_PRIVATE_KEY,
          },
          enableLineaEstimateGas: false,
          isCalldataEnabled: DEFAULT_CALLDATA_ENABLED,
          isEOAEnabled: DEFAULT_EOA_ENABLED,
          l2MessageTreeDepth: DEFAULT_L2_MESSAGE_TREE_DEPTH,
          listener: {
            blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
            initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
            maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
            maxFetchMessagesFromDb: DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
            pollingInterval: DEFAULT_LISTENER_INTERVAL,
          },
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
          rpcUrl: TEST_RPC_URL,
        },
        l2L1AutoClaimEnabled: false,
        loggerOptions: undefined,
      });
    });

    it("should return the config when some optional parameters are passed.", () => {
      const config = getConfig({
        l1Options: {
          rpcUrl: TEST_RPC_URL,
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
          listener: {
            pollingInterval: DEFAULT_LISTENER_INTERVAL + 1000,
          },
          claiming: {
            signerPrivateKey: TEST_L1_SIGNER_PRIVATE_KEY,
            feeRecipientAddress: TEST_ADDRESS_1,
          },
        },
        l2Options: {
          rpcUrl: TEST_RPC_URL,
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
          enableLineaEstimateGas: true,
          listener: {
            pollingInterval: DEFAULT_LISTENER_INTERVAL + 1000,
          },
          claiming: {
            signerPrivateKey: TEST_L2_SIGNER_PRIVATE_KEY,
            feeRecipientAddress: TEST_ADDRESS_2,
          },
        },
        l1L2AutoClaimEnabled: true,
        l2L1AutoClaimEnabled: true,
        databaseOptions: {
          type: "postgres",
        },
        databaseCleanerOptions: {
          enabled: true,
        },
      });
      expect(config).toStrictEqual({
        databaseCleanerConfig: {
          cleaningInterval: DEFAULT_DB_CLEANING_INTERVAL,
          daysBeforeNowToDelete: DEFAULT_DB_DAYS_BEFORE_NOW_TO_DELETE,
          enabled: true,
        },
        databaseOptions: {
          type: "postgres",
        },
        l1Config: {
          claiming: {
            feeRecipientAddress: TEST_ADDRESS_1,
            gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
            isMaxGasFeeEnforced: DEFAULT_ENFORCE_MAX_GAS_FEE,
            maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
            maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
            maxNonceDiff: DEFAULT_MAX_NONCE_DIFF,
            maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
            maxTxRetries: DEFAULT_MAX_TX_RETRIES,
            messageSubmissionTimeout: DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
            profitMargin: DEFAULT_PROFIT_MARGIN,
            retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
            signerPrivateKey: TEST_L1_SIGNER_PRIVATE_KEY,
          },
          isCalldataEnabled: DEFAULT_CALLDATA_ENABLED,
          isEOAEnabled: DEFAULT_EOA_ENABLED,
          listener: {
            blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
            initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
            maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
            maxFetchMessagesFromDb: DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
            pollingInterval: DEFAULT_LISTENER_INTERVAL + 1000,
          },
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
          rpcUrl: TEST_RPC_URL,
        },
        l1L2AutoClaimEnabled: true,
        l2Config: {
          claiming: {
            feeRecipientAddress: TEST_ADDRESS_2,
            gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
            isMaxGasFeeEnforced: DEFAULT_ENFORCE_MAX_GAS_FEE,
            maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
            maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
            maxNonceDiff: DEFAULT_MAX_NONCE_DIFF,
            maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
            maxTxRetries: DEFAULT_MAX_TX_RETRIES,
            messageSubmissionTimeout: DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
            profitMargin: DEFAULT_PROFIT_MARGIN,
            retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
            signerPrivateKey: TEST_L2_SIGNER_PRIVATE_KEY,
          },
          enableLineaEstimateGas: true,
          isCalldataEnabled: DEFAULT_CALLDATA_ENABLED,
          isEOAEnabled: DEFAULT_EOA_ENABLED,
          l2MessageTreeDepth: DEFAULT_L2_MESSAGE_TREE_DEPTH,
          listener: {
            blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
            initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
            maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
            maxFetchMessagesFromDb: DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
            pollingInterval: DEFAULT_LISTENER_INTERVAL + 1000,
          },
          messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
          rpcUrl: TEST_RPC_URL,
        },
        l2L1AutoClaimEnabled: true,
        loggerOptions: undefined,
      });
    });
  });
});
