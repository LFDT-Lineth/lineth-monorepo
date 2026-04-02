import {
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_L1_SIGNER_PRIVATE_KEY,
  TEST_L2_SIGNER_PRIVATE_KEY,
  TEST_RPC_URL,
} from "./constants";
import {
  mockCalldataDecoder,
  mockErrorParser,
  mockEthereumGasProvider,
  mockL2ClaimTransactionSizeCalculator,
  mockL2MessageServiceClient,
  mockL2MessageServiceLogClient,
  mockLineaProvider,
  mockLineaRollupClient,
  mockLineaRollupLogClient,
  mockMessageRepository,
  mockNonceManager,
  mockProvider,
  mockReceiptPoller,
  mockSponsorshipMetricsUpdater,
  mockTransactionMetricsUpdater,
  mockTransactionRetrier,
  mockTransactionValidationService,
} from "./mocks";
import { PostmanConfig, PostmanOptions } from "../../application/postman/app/config/config";
import { L1ToL2Deps } from "../../application/postman/app/L1ToL2App";
import { L2ToL1Deps } from "../../application/postman/app/L2ToL1App";
import {
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_INITIAL_FROM_BLOCK,
  DEFAULT_L2_MESSAGE_TREE_DEPTH,
  DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
  DEFAULT_LISTENER_INTERVAL,
  DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
  DEFAULT_MAX_BUMPS_PER_CYCLE,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
  DEFAULT_MAX_NONCE_DIFF,
  DEFAULT_MAX_NUMBER_OF_RETRIES,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_MAX_RETRY_CYCLES,
  DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
  DEFAULT_PROFIT_MARGIN,
  DEFAULT_RETRY_DELAY_IN_SECONDS,
} from "../../core/constants";

export function buildTestPostmanOptions(overrides?: Partial<PostmanOptions>): PostmanOptions {
  return {
    l1Options: {
      rpcUrl: TEST_RPC_URL,
      messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
      listener: {},
      claiming: { signer: { type: "private-key" as const, privateKey: TEST_L1_SIGNER_PRIVATE_KEY } },
    },
    l2Options: {
      rpcUrl: TEST_RPC_URL,
      messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
      listener: {},
      claiming: { signer: { type: "private-key" as const, privateKey: TEST_L2_SIGNER_PRIVATE_KEY } },
    },
    l1L2AutoClaimEnabled: false,
    l2L1AutoClaimEnabled: false,
    databaseOptions: { type: "postgres" as const },
    ...overrides,
  };
}

export function buildTestPostmanConfig(overrides?: Partial<PostmanConfig>): PostmanConfig {
  return {
    l1Config: {
      rpcUrl: TEST_RPC_URL,
      messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
      isEOAEnabled: true,
      isCalldataEnabled: false,
      listener: {
        pollingInterval: DEFAULT_LISTENER_INTERVAL,
        receiptPollingInterval: DEFAULT_LISTENER_INTERVAL,
        maxFetchMessagesFromDb: DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
        maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
        initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
        blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
      },
      claiming: {
        signer: { type: "private-key" as const, privateKey: TEST_L1_SIGNER_PRIVATE_KEY },
        messageSubmissionTimeout: DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
        maxNonceDiff: DEFAULT_MAX_NONCE_DIFF,
        maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
        gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
        isMaxGasFeeEnforced: false,
        profitMargin: DEFAULT_PROFIT_MARGIN,
        maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
        retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
        maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
        maxBumpsPerCycle: DEFAULT_MAX_BUMPS_PER_CYCLE,
        maxRetryCycles: DEFAULT_MAX_RETRY_CYCLES,
        isPostmanSponsorshipEnabled: DEFAULT_ENABLE_POSTMAN_SPONSORING,
        maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
      },
    },
    l2Config: {
      rpcUrl: TEST_RPC_URL,
      messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
      isEOAEnabled: true,
      isCalldataEnabled: false,
      l2MessageTreeDepth: DEFAULT_L2_MESSAGE_TREE_DEPTH,
      enableLineaEstimateGas: false,
      listener: {
        pollingInterval: DEFAULT_LISTENER_INTERVAL,
        receiptPollingInterval: DEFAULT_LISTENER_INTERVAL,
        maxFetchMessagesFromDb: DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
        maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
        initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
        blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
      },
      claiming: {
        signer: { type: "private-key" as const, privateKey: TEST_L2_SIGNER_PRIVATE_KEY },
        messageSubmissionTimeout: DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
        maxNonceDiff: DEFAULT_MAX_NONCE_DIFF,
        maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
        gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
        isMaxGasFeeEnforced: false,
        profitMargin: DEFAULT_PROFIT_MARGIN,
        maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
        retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
        maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
        maxBumpsPerCycle: DEFAULT_MAX_BUMPS_PER_CYCLE,
        maxRetryCycles: DEFAULT_MAX_RETRY_CYCLES,
        isPostmanSponsorshipEnabled: DEFAULT_ENABLE_POSTMAN_SPONSORING,
        maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
      },
    },
    l1L2AutoClaimEnabled: false,
    l2L1AutoClaimEnabled: false,
    databaseOptions: { type: "postgres" as const },
    databaseCleanerConfig: {
      enabled: false,
      cleaningInterval: 43200000,
      daysBeforeNowToDelete: 14,
    },
    apiConfig: { port: 3000 },
    ...overrides,
  };
}

export function buildL1ToL2Deps(overrides?: Partial<L1ToL2Deps>): L1ToL2Deps {
  const config = buildTestPostmanConfig();
  return {
    l1LogClient: mockLineaRollupLogClient(),
    l1Provider: mockProvider(),
    l2MessageServiceClient: mockL2MessageServiceClient(),
    l2Provider: mockLineaProvider(),
    l2NonceManager: mockNonceManager(),
    l2TransactionRetrier: mockTransactionRetrier(),
    l2ReceiptPoller: mockReceiptPoller(),
    messageRepository: mockMessageRepository(),
    calldataDecoder: mockCalldataDecoder(),
    transactionValidationService: mockTransactionValidationService(),
    transactionSizeCalculator: mockL2ClaimTransactionSizeCalculator(),
    sponsorshipMetricsUpdater: mockSponsorshipMetricsUpdater(),
    transactionMetricsUpdater: mockTransactionMetricsUpdater(),
    errorParser: mockErrorParser(),
    l1Config: config.l1Config,
    l2Config: config.l2Config,
    ...overrides,
  };
}

export function buildL2ToL1Deps(overrides?: Partial<L2ToL1Deps>): L2ToL1Deps {
  const config = buildTestPostmanConfig();
  return {
    l2LogClient: mockL2MessageServiceLogClient(),
    l2Provider: mockProvider(),
    lineaRollupClient: mockLineaRollupClient(),
    l1Provider: mockProvider(),
    l1NonceManager: mockNonceManager(),
    l1TransactionRetrier: mockTransactionRetrier(),
    l1ReceiptPoller: mockReceiptPoller(),
    messageRepository: mockMessageRepository(),
    l1GasProvider: mockEthereumGasProvider(),
    transactionValidationService: mockTransactionValidationService(),
    calldataDecoder: mockCalldataDecoder(),
    sponsorshipMetricsUpdater: mockSponsorshipMetricsUpdater(),
    transactionMetricsUpdater: mockTransactionMetricsUpdater(),
    errorParser: mockErrorParser(),
    l1Config: config.l1Config,
    l2Config: config.l2Config,
    ...overrides,
  };
}

export const TEST_ENV_VARS: Record<string, string> = {
  L1_RPC_URL: "http://localhost:8445",
  L2_RPC_URL: "http://localhost:8545",
  L1_CONTRACT_ADDRESS: TEST_CONTRACT_ADDRESS_1,
  L2_CONTRACT_ADDRESS: TEST_CONTRACT_ADDRESS_2,
  L1_SIGNER_PRIVATE_KEY: TEST_L1_SIGNER_PRIVATE_KEY,
  L2_SIGNER_PRIVATE_KEY: TEST_L2_SIGNER_PRIVATE_KEY,
  L1_L2_AUTO_CLAIM_ENABLED: "true",
  L2_L1_AUTO_CLAIM_ENABLED: "true",
  L1_L2_EOA_ENABLED: "true",
  L2_L1_EOA_ENABLED: "true",
  POSTGRES_HOST: "localhost",
  POSTGRES_PORT: "5432",
  POSTGRES_USER: "test_user",
  POSTGRES_PASSWORD: "test_password",
  POSTGRES_DB: "test_db",
};
