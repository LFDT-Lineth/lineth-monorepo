import { Direction, MessageSent } from "@consensys/linea-sdk";
import { L1NetworkConfig, L2NetworkConfig } from "../../application/postman/app/config/config";
import { Message, MessageProps } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import {
  DEFAULT_INITIAL_FROM_BLOCK,
  DEFAULT_L2_MESSAGE_TREE_DEPTH,
  DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
  DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_NONCE_DIFF,
  DEFAULT_MAX_NUMBER_OF_RETRIES,
  DEFAULT_MAX_TX_RETRIES,
  DEFAULT_PROFIT_MARGIN,
  DEFAULT_RETRY_DELAY_IN_SECONDS,
  ZERO_ADDRESS,
  ZERO_HASH,
} from "../../core/constants";

export const TEST_L1_SIGNER_PRIVATE_KEY = "0x0000000000000000000000000000000000000000000000000000000000000001";
export const TEST_L2_SIGNER_PRIVATE_KEY = "0x0000000000000000000000000000000000000000000000000000000000000002";

export const TEST_ADDRESS_1 = "0x0000000000000000000000000000000000000001";
export const TEST_ADDRESS_2 = "0x0000000000000000000000000000000000000002";

export const TEST_CONTRACT_ADDRESS_1 = "0x1000000000000000000000000000000000000000";
export const TEST_CONTRACT_ADDRESS_2 = "0x2000000000000000000000000000000000000000";

export const TEST_MESSAGE_HASH = "0x1010101010101010101010101010101010101010101010101010101010101010";
export const TEST_MESSAGE_HASH_2 = "0x1010101010101010101010101010101010101010101010101010101010101020";

export const TEST_TRANSACTION_HASH = "0x2020202020202020202020202020202020202020202020202020202020202020";

export const TEST_RPC_URL = "http://localhost:8545";

export const DEFAULT_MAX_FEE_PER_GAS = 100000000000n;

export const testMessage = new Message({
  messageSender: TEST_ADDRESS_1,
  destination: TEST_CONTRACT_ADDRESS_1,
  fee: 100000000000n,
  value: 0n,
  messageNonce: 10n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  sentBlockNumber: 10,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.SENT,
  claimNumberOfRetry: 0,
  compressedTransactionSize: 100,
});

export const testAnchoredMessage = new Message({
  messageSender: TEST_ADDRESS_1,
  destination: TEST_CONTRACT_ADDRESS_1,
  fee: 1000000000000000n,
  value: 0n,
  messageNonce: 10n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  sentBlockNumber: 10,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.ANCHORED,
  claimNumberOfRetry: 0,
});

export const testZeroFeeAnchoredMessage = new Message({
  messageSender: TEST_ADDRESS_1,
  destination: TEST_CONTRACT_ADDRESS_1,
  fee: 0n,
  value: 0n,
  messageNonce: 10n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  sentBlockNumber: 10,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.ANCHORED,
  claimNumberOfRetry: 0,
});

export const testUnderpricedAnchoredMessage = new Message({
  messageSender: TEST_ADDRESS_1,
  destination: TEST_CONTRACT_ADDRESS_1,
  fee: 1000000n,
  value: 0n,
  messageNonce: 10n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  sentBlockNumber: 10,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.ANCHORED,
  claimNumberOfRetry: 0,
});

export const testPendingMessage = new Message({
  messageSender: TEST_ADDRESS_1,
  destination: TEST_CONTRACT_ADDRESS_1,
  fee: 100000000000n,
  value: 0n,
  messageNonce: 10n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  sentBlockNumber: 10,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.PENDING,
  claimNumberOfRetry: 0,
  claimTxHash: TEST_TRANSACTION_HASH,
  updatedAt: new Date(2024, 1, 1),
});

export const testPendingMessage2 = new Message({
  messageSender: TEST_ADDRESS_1,
  destination: TEST_CONTRACT_ADDRESS_1,
  fee: 100000000000n,
  value: 0n,
  messageNonce: 10n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH_2,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  sentBlockNumber: 10,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.PENDING,
  claimNumberOfRetry: 0,
  claimTxHash: TEST_TRANSACTION_HASH,
  updatedAt: new Date(2024, 1, 1),
});

export const testClaimedMessage = new Message({
  messageSender: TEST_ADDRESS_1,
  destination: TEST_CONTRACT_ADDRESS_1,
  fee: 100000000000n,
  value: 0n,
  messageNonce: 10n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  sentBlockNumber: 10,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.CLAIMED_SUCCESS,
  claimNumberOfRetry: 0,
});

export const rejectedMessageProps: MessageProps = {
  messageHash: ZERO_HASH,
  messageSender: ZERO_ADDRESS,
  destination: ZERO_ADDRESS,
  fee: 0n,
  value: 0n,
  messageNonce: 0n,
  calldata: "0x",
  contractAddress: ZERO_ADDRESS,
  sentBlockNumber: 0,
  direction: Direction.L1_TO_L2,
  status: MessageStatus.SENT,
  claimNumberOfRetry: 0,
};

export const testMessageSentEvent: MessageSent = {
  blockNumber: 51,
  logIndex: 1,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  transactionHash: TEST_TRANSACTION_HASH,
  messageSender: TEST_ADDRESS_1,
  destination: TEST_ADDRESS_2,
  fee: 0n,
  value: 0n,
  messageNonce: 1n,
  calldata: "0x",
  messageHash: TEST_MESSAGE_HASH,
};

export const testMessageSentEventWithCallData: MessageSent = {
  blockNumber: 51,
  logIndex: 0,
  contractAddress: TEST_CONTRACT_ADDRESS_1,
  transactionHash: TEST_TRANSACTION_HASH,
  messageSender: TEST_ADDRESS_1,
  destination: TEST_ADDRESS_2,
  fee: 0n,
  value: 0n,
  messageNonce: 1n,
  calldata: "0x1111111111",
  messageHash: TEST_MESSAGE_HASH,
};

export const testL1NetworkConfig: L1NetworkConfig = {
  claiming: {
    signerPrivateKey: TEST_L1_SIGNER_PRIVATE_KEY,
    messageSubmissionTimeout: 300_000,
    maxFeePerGasCap: 100_000_000n,
    gasEstimationPercentile: 15,
    maxNonceDiff: DEFAULT_MAX_NONCE_DIFF,
    isMaxGasFeeEnforced: false,
    profitMargin: DEFAULT_PROFIT_MARGIN,
    maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
    retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
    maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
    maxTxRetries: DEFAULT_MAX_TX_RETRIES,
  },
  listener: {
    pollingInterval: 4000,
    maxFetchMessagesFromDb: 3,
    initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
    blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
    maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
  },
  rpcUrl: "http://localhost:8445",
  messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
  isEOAEnabled: true,
  isCalldataEnabled: false,
};

export const testL2NetworkConfig: L2NetworkConfig = {
  claiming: {
    signerPrivateKey: TEST_L2_SIGNER_PRIVATE_KEY,
    messageSubmissionTimeout: 300_000,
    maxFeePerGasCap: 100_000_000n,
    gasEstimationPercentile: 15,
    maxNonceDiff: 10,
    maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
    isMaxGasFeeEnforced: false,
    profitMargin: DEFAULT_PROFIT_MARGIN,
    maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
    retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
    maxTxRetries: DEFAULT_MAX_TX_RETRIES,
  },
  listener: {
    pollingInterval: 100,
    maxFetchMessagesFromDb: 3,
    initialFromBlock: 10,
    blockConfirmation: DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
    maxBlocksToFetchLogs: DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
  },
  rpcUrl: "http://localhost:8545",
  messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
  isCalldataEnabled: false,
  isEOAEnabled: true,
  l2MessageTreeDepth: DEFAULT_L2_MESSAGE_TREE_DEPTH,
  enableLineaEstimateGas: false,
};
