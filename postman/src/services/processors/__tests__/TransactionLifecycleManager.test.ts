import { describe, it, beforeEach, expect, jest } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { ITransactionProvider } from "../../../core/clients/blockchain/IProvider";
import { Message } from "../../../core/entities/Message";
import { Direction, MessageStatus, OnChainMessageStatus } from "../../../core/enums";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { IMessageStatusReader } from "../../../core/services/contracts/IMessageServiceContract";
import { IReceiptPoller } from "../../../core/services/IReceiptPoller";
import { ITransactionRetrier } from "../../../core/services/ITransactionRetrier";
import {
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
  TEST_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_1,
} from "../../../utils/testing/constants";
import { TestLogger, generateReceipt, generateSubmission } from "../../../utils/testing/helpers";
import { TransactionLifecycleManager } from "../TransactionLifecycleManager";

const CANCEL_TX_HASH = "0xcacecacecacecacecacecacecacecacecacecacecacecacecacecacecacecace" as const;
const BUMP_TX_HASH = "0xbumpbumpbumpbumpbumpbumpbumpbumpbumpbumpbumpbumpbumpbumpbumpbump" as const;

function makePendingMessage(overrides: Partial<ConstructorParameters<typeof Message>[0]> = {}): Message {
  return new Message({
    messageSender: TEST_ADDRESS_1,
    destination: TEST_CONTRACT_ADDRESS_1,
    fee: 100_000_000_000n,
    value: 0n,
    messageNonce: 10n,
    calldata: "0x",
    messageHash: TEST_MESSAGE_HASH,
    contractAddress: TEST_CONTRACT_ADDRESS_1,
    sentBlockNumber: 10,
    direction: Direction.L1_TO_L2,
    status: MessageStatus.PENDING,
    claimNumberOfRetry: 2,
    claimCycleCount: 1,
    claimTxHash: TEST_TRANSACTION_HASH,
    claimTxNonce: 42,
    claimTxMaxFeePerGas: 100_000_000_000n,
    claimTxMaxPriorityFeePerGas: 1_000_000_000n,
    ...overrides,
  });
}

function createManager() {
  const messageServiceContract = mock<IMessageStatusReader>();
  const provider = mock<ITransactionProvider>();
  const transactionRetrier = mock<ITransactionRetrier>();
  const receiptPoller = mock<IReceiptPoller>();
  const messageRepository = mock<IMessageRepository>();
  const logger = new TestLogger(TransactionLifecycleManager.name);

  const manager = new TransactionLifecycleManager(
    messageServiceContract,
    provider,
    transactionRetrier,
    receiptPoller,
    messageRepository,
    { receiptPollingTimeout: 120_000, receiptPollingInterval: 0 },
    logger,
  );

  return { manager, messageServiceContract, provider, transactionRetrier, receiptPoller, messageRepository };
}

describe("TransactionLifecycleManager", () => {
  let manager: TransactionLifecycleManager;
  let messageServiceContract: ReturnType<typeof mock<IMessageStatusReader>>;
  let provider: ReturnType<typeof mock<ITransactionProvider>>;
  let transactionRetrier: ReturnType<typeof mock<ITransactionRetrier>>;
  let receiptPoller: ReturnType<typeof mock<IReceiptPoller>>;
  let messageRepository: ReturnType<typeof mock<IMessageRepository>>;

  beforeEach(() => {
    jest.resetAllMocks();
    ({ manager, messageServiceContract, provider, transactionRetrier, receiptPoller, messageRepository } =
      createManager());
  });

  // ---------------------------------------------------------------------------
  // retryWithBump
  // ---------------------------------------------------------------------------
  describe("retryWithBump", () => {
    it("should return the receipt when the message is already claimed on-chain", async () => {
      const message = makePendingMessage();
      const receipt = generateReceipt();

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.CLAIMED);
      provider.getTransactionReceipt.mockResolvedValue(receipt);

      const result = await manager.retryWithBump(message);

      expect(result).toEqual(receipt);
      expect(provider.getTransactionReceipt).toHaveBeenCalledWith(TEST_TRANSACTION_HASH);
    });

    it("should return null and log a warning when already claimed but receipt unavailable", async () => {
      const message = makePendingMessage();

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.CLAIMED);
      provider.getTransactionReceipt.mockResolvedValue(null);

      const result = await manager.retryWithBump(message);

      expect(result).toBeNull();
    });

    it("should bump the fee, update the message, and poll for the receipt", async () => {
      const message = makePendingMessage();
      const submission = generateSubmission({ hash: BUMP_TX_HASH });
      const receipt = generateReceipt({ hash: BUMP_TX_HASH });

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      transactionRetrier.retryWithHigherFee.mockResolvedValue(submission);
      receiptPoller.poll.mockResolvedValue(receipt);

      const result = await manager.retryWithBump(message);

      expect(transactionRetrier.retryWithHigherFee).toHaveBeenCalledWith(TEST_TRANSACTION_HASH, 3); // claimNumberOfRetry+1
      expect(messageRepository.updateMessage).toHaveBeenCalledTimes(1);
      expect(message.claimTxHash).toBe(BUMP_TX_HASH);
      expect(message.claimTxNonce).toBe(42);
      expect(message.claimNumberOfRetry).toBe(3);
      expect(receiptPoller.poll).toHaveBeenCalledWith(BUMP_TX_HASH, 120_000, 0);
      expect(result).toEqual(receipt);
    });

    it("should update message fields from the bumped transaction", async () => {
      const message = makePendingMessage({ claimNumberOfRetry: 0 });
      const submission = generateSubmission({
        hash: BUMP_TX_HASH,
        nonce: 99,
        gasLimit: 75_000n,
        maxFeePerGas: 300_000_000_000n,
        maxPriorityFeePerGas: 3_000_000_000n,
      });

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      transactionRetrier.retryWithHigherFee.mockResolvedValue(submission);
      receiptPoller.poll.mockResolvedValue(generateReceipt({ hash: BUMP_TX_HASH }));

      await manager.retryWithBump(message);

      expect(messageRepository.updateMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          claimTxHash: BUMP_TX_HASH,
          claimTxNonce: 99,
          claimTxGasLimit: 75_000,
          claimTxMaxFeePerGas: 300_000_000_000n,
          claimTxMaxPriorityFeePerGas: 3_000_000_000n,
          claimNumberOfRetry: 1,
        }),
      );
    });

    it("should return null and not throw when retryWithHigherFee rejects", async () => {
      const message = makePendingMessage();

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      transactionRetrier.retryWithHigherFee.mockRejectedValue(new Error("replacement underpriced"));

      const result = await manager.retryWithBump(message);

      expect(result).toBeNull();
      expect(messageRepository.updateMessage).not.toHaveBeenCalled();
    });

    it("should return null and not throw when getMessageStatus rejects", async () => {
      const message = makePendingMessage();

      messageServiceContract.getMessageStatus.mockRejectedValue(new Error("rpc error"));

      const result = await manager.retryWithBump(message);

      expect(result).toBeNull();
    });

    it("should still poll the new tx and return the receipt when the DB update fails after fee bump", async () => {
      const message = makePendingMessage();
      const submission = generateSubmission({ hash: BUMP_TX_HASH });
      const receipt = generateReceipt({ hash: BUMP_TX_HASH });

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      transactionRetrier.retryWithHigherFee.mockResolvedValue(submission);
      messageRepository.updateMessage.mockRejectedValue(new Error("DB connection lost"));
      receiptPoller.poll.mockResolvedValue(receipt);

      const result = await manager.retryWithBump(message);

      // Must poll the bumped tx hash — not return null — so the persister can recover state
      expect(receiptPoller.poll).toHaveBeenCalledWith(BUMP_TX_HASH, 120_000, 0);
      expect(result).toEqual(receipt);
    });
  });

  // ---------------------------------------------------------------------------
  // cancelAndResetMessage
  // ---------------------------------------------------------------------------
  describe("cancelAndResetMessage", () => {
    it("should return the receipt when the message is already claimed on-chain", async () => {
      const message = makePendingMessage();
      const receipt = generateReceipt();

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.CLAIMED);
      provider.getTransactionReceipt.mockResolvedValue(receipt);

      const result = await manager.cancelAndResetMessage(message);

      expect(result).toEqual(receipt);
    });

    it("should return null when already claimed but receipt unavailable", async () => {
      const message = makePendingMessage();

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.CLAIMED);
      provider.getTransactionReceipt.mockResolvedValue(null);

      const result = await manager.cancelAndResetMessage(message);

      expect(result).toBeNull();
    });

    it("should cancel the stuck tx, poll for the cancel receipt, reset the message, and update the DB", async () => {
      const message = makePendingMessage();

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      transactionRetrier.cancelTransaction.mockResolvedValue(CANCEL_TX_HASH);
      receiptPoller.poll.mockResolvedValue(generateReceipt({ hash: CANCEL_TX_HASH }));

      const result = await manager.cancelAndResetMessage(message);

      expect(transactionRetrier.cancelTransaction).toHaveBeenCalledWith(42, {
        maxFeePerGas: 100_000_000_000n,
        maxPriorityFeePerGas: 1_000_000_000n,
      });
      expect(receiptPoller.poll).toHaveBeenCalledWith(CANCEL_TX_HASH, 120_000, 0);
      expect(message.claimTxHash).toBeUndefined();
      expect(message.claimTxNonce).toBeUndefined();
      expect(message.status).toBe(MessageStatus.SENT);
      expect(message.claimNumberOfRetry).toBe(0);
      expect(message.claimCycleCount).toBe(2); // incremented
      expect(messageRepository.updateMessage).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.SENT }),
      );
      expect(result).toBeNull();
    });

    it("should pass undefined stuckFees when message has no gas price recorded", async () => {
      const message = makePendingMessage({
        claimTxMaxFeePerGas: undefined,
        claimTxMaxPriorityFeePerGas: undefined,
      });

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      transactionRetrier.cancelTransaction.mockResolvedValue(CANCEL_TX_HASH);
      receiptPoller.poll.mockResolvedValue(generateReceipt({ hash: CANCEL_TX_HASH }));

      await manager.cancelAndResetMessage(message);

      expect(transactionRetrier.cancelTransaction).toHaveBeenCalledWith(42, undefined);
    });

    it("should skip cancelTransaction when claimTxNonce is undefined", async () => {
      const message = makePendingMessage({ claimTxNonce: undefined });

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);

      const result = await manager.cancelAndResetMessage(message);

      expect(transactionRetrier.cancelTransaction).not.toHaveBeenCalled();
      expect(receiptPoller.poll).not.toHaveBeenCalled();
      expect(message.status).toBe(MessageStatus.SENT);
      expect(messageRepository.updateMessage).toHaveBeenCalledTimes(1);
      expect(result).toBeNull();
    });

    it("should return null and not throw when cancelTransaction rejects", async () => {
      const message = makePendingMessage();

      messageServiceContract.getMessageStatus.mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      transactionRetrier.cancelTransaction.mockRejectedValue(new Error("nonce too low"));

      const result = await manager.cancelAndResetMessage(message);

      expect(result).toBeNull();
      expect(messageRepository.updateMessage).not.toHaveBeenCalled();
    });

    it("should return null and not throw when getMessageStatus rejects", async () => {
      const message = makePendingMessage();

      messageServiceContract.getMessageStatus.mockRejectedValue(new Error("network error"));

      const result = await manager.cancelAndResetMessage(message);

      expect(result).toBeNull();
    });
  });
});
