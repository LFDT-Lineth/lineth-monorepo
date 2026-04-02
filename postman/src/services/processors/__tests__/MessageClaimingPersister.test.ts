import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { ISponsorshipMetricsUpdater, ITransactionMetricsUpdater } from "../../../../src/core/metrics";
import { ITransactionProvider, IBlockProvider } from "../../../core/clients/blockchain/IProvider";
import { Message } from "../../../core/entities/Message";
import { Direction, OnChainMessageStatus, MessageStatus } from "../../../core/enums";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { IMessageStatusReader, IRateLimitChecker } from "../../../core/services/contracts/IMessageServiceContract";
import { IReceiptPoller } from "../../../core/services/IReceiptPoller";
import { ITransactionRetrier } from "../../../core/services/ITransactionRetrier";
import { IMessageClaimingPersister } from "../../../core/services/processors/IMessageClaimingPersister";
import { TransactionReceipt, TransactionSubmission } from "../../../core/types";
import {
  TEST_TRANSACTION_HASH,
  testL2NetworkConfig,
  testPendingMessage,
  testPendingMessage2,
} from "../../../utils/testing/constants";
import { generateReceipt, TestLogger } from "../../../utils/testing/helpers";
import { MessageClaimingPersister } from "../MessageClaimingPersister";
import { ReceiptStatusResolver } from "../ReceiptStatusResolver";
import { TransactionLifecycleManager } from "../TransactionLifecycleManager";

const generateTransactionReceipt = (overrides: Partial<TransactionReceipt> = {}): TransactionReceipt => ({
  hash: TEST_TRANSACTION_HASH,
  blockNumber: 100,
  status: "success",
  gasUsed: 50_000n,
  gasPrice: 100_000_000_000n,
  logs: [],
  ...overrides,
});

const generateTransactionSubmission = (overrides: Partial<TransactionSubmission> = {}): TransactionSubmission => ({
  hash: TEST_TRANSACTION_HASH,
  nonce: 1,
  gasLimit: 50_000n,
  maxFeePerGas: 100_000_000_000n,
  maxPriorityFeePerGas: 1_000_000_000n,
  ...overrides,
});

function createPersister(
  overrides: {
    direction?: Direction;
    messageSubmissionTimeout?: number;
    maxBumpsPerCycle?: number;
    maxCycles?: number;
    receiptPollingTimeout?: number;
    receiptPollingInterval?: number;
  } = {},
) {
  const databaseService = mock<IMessageRepository>();
  const messageServiceContract = mock<IMessageStatusReader & IRateLimitChecker>();
  const sponsorshipMetricsUpdater = mock<ISponsorshipMetricsUpdater>();
  const transactionMetricsUpdater = mock<ITransactionMetricsUpdater>();
  const provider = mock<ITransactionProvider & IBlockProvider>();
  const transactionRetrier = mock<ITransactionRetrier>();
  const receiptPoller = mock<IReceiptPoller>();
  const logger = new TestLogger(MessageClaimingPersister.name);

  const direction = overrides.direction ?? Direction.L1_TO_L2;

  const transactionLifecycleManager = new TransactionLifecycleManager(
    messageServiceContract,
    provider,
    transactionRetrier,
    receiptPoller,
    databaseService,
    {
      receiptPollingTimeout: overrides.receiptPollingTimeout ?? 120_000,
      receiptPollingInterval: overrides.receiptPollingInterval ?? 0,
    },
    new TestLogger(TransactionLifecycleManager.name),
  );

  const receiptStatusResolver = new ReceiptStatusResolver(
    databaseService,
    messageServiceContract,
    provider,
    sponsorshipMetricsUpdater,
    transactionMetricsUpdater,
    { direction },
    new TestLogger(ReceiptStatusResolver.name),
  );

  const persister = new MessageClaimingPersister(
    databaseService,
    provider,
    transactionLifecycleManager,
    receiptStatusResolver,
    {
      direction,
      messageSubmissionTimeout:
        overrides.messageSubmissionTimeout ?? testL2NetworkConfig.claiming.messageSubmissionTimeout,
      maxBumpsPerCycle: overrides.maxBumpsPerCycle ?? testL2NetworkConfig.claiming.maxBumpsPerCycle,
      maxCycles: overrides.maxCycles ?? testL2NetworkConfig.claiming.maxRetryCycles,
    },
    logger,
  );

  return {
    persister,
    databaseService,
    messageServiceContract,
    sponsorshipMetricsUpdater,
    transactionMetricsUpdater,
    provider,
    transactionRetrier,
    receiptPoller,
    logger,
  };
}

describe("TestMessageClaimingPersister", () => {
  let messageClaimingPersister: IMessageClaimingPersister;
  let databaseService: ReturnType<typeof createPersister>["databaseService"];
  let messageServiceContract: ReturnType<typeof createPersister>["messageServiceContract"];
  let sponsorshipMetricsUpdater: ReturnType<typeof createPersister>["sponsorshipMetricsUpdater"];
  let transactionMetricsUpdater: ReturnType<typeof createPersister>["transactionMetricsUpdater"];
  let provider: ReturnType<typeof createPersister>["provider"];
  let transactionRetrier: ReturnType<typeof createPersister>["transactionRetrier"];
  let receiptPoller: ReturnType<typeof createPersister>["receiptPoller"];
  let logger: TestLogger;

  beforeEach(() => {
    const ctx = createPersister();
    messageClaimingPersister = ctx.persister;
    databaseService = ctx.databaseService;
    messageServiceContract = ctx.messageServiceContract;
    sponsorshipMetricsUpdater = ctx.sponsorshipMetricsUpdater;
    transactionMetricsUpdater = ctx.transactionMetricsUpdater;
    provider = ctx.provider;
    transactionRetrier = ctx.transactionRetrier;
    receiptPoller = ctx.receiptPoller;
    logger = ctx.logger;

    jest.useFakeTimers();
    jest.setSystemTime(new Date());
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("process", () => {
    it("Should early return immediately if no pending message found", async () => {
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(null);
      const getReceiptSpy = jest.spyOn(provider, "getTransactionReceipt");

      await messageClaimingPersister.process();

      expect(getReceiptSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as error if getTransactionReceipt throws error", async () => {
      const getTxReceiptError = new Error("error for testing");
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(new Message(testPendingMessage));
      jest.spyOn(provider, "getTransactionReceipt").mockRejectedValue(getTxReceiptError);
      const loggerErrorSpy = jest.spyOn(logger, "error");

      await messageClaimingPersister.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error processing pending message.", {
        error: getTxReceiptError,
        messageHash: testPendingMessage.messageHash,
      });
    });

    it("Should log as info and update message as claimed success if successful", async () => {
      const txReceipt = generateTransactionReceipt({ status: "success" });
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.CLAIMED_SUCCESS }),
      );
    });

    it("Should return and update message as sent if receipt status is reverted and rate limit exceeded", async () => {
      const txReceipt = generateTransactionReceipt({ status: "reverted" });
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(new Message(testPendingMessage));
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      jest.spyOn(messageServiceContract, "isRateLimitExceededError").mockResolvedValue(true);
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(expect.objectContaining({ status: MessageStatus.SENT }));
    });

    it("Should log as warning and update message as claim reverted if receipt status is reverted", async () => {
      const txReceipt = generateTransactionReceipt({ status: "reverted" });
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(new Message(testPendingMessage));
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      jest.spyOn(messageServiceContract, "isRateLimitExceededError").mockResolvedValue(false);
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.CLAIMED_REVERTED }),
      );
    });

    it("Should update message as claimed if message claimed on-chain and receipt found on retry", async () => {
      const retryTxReceipt = generateTransactionReceipt({ status: "success" });
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValueOnce(null).mockResolvedValueOnce(retryTxReceipt);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.CLAIMED_SUCCESS }),
      );
    });

    it("Should return and log as warning if message is claimed but receipt returned as null", async () => {
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(0);
    });

    it("Should bump fee and update DB when message is claimable and retry tx succeeds", async () => {
      const retryTxReceipt = generateTransactionReceipt({ status: "success" });
      const retryTxResponse = generateTransactionSubmission({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(transactionRetrier, "retryWithHigherFee").mockResolvedValue(retryTxResponse);
      jest.spyOn(receiptPoller, "poll").mockResolvedValue(retryTxReceipt);
      jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositoryUpdateSpy).toHaveBeenNthCalledWith(1, expect.objectContaining({ claimNumberOfRetry: 1 }));
    });

    it("Should return null when retry tx throws error", async () => {
      const ctx = createPersister({ messageSubmissionTimeout: 0, maxBumpsPerCycle: 5, maxCycles: 2 });
      messageClaimingPersister = ctx.persister;
      provider = ctx.provider;
      databaseService = ctx.databaseService;
      messageServiceContract = ctx.messageServiceContract;
      transactionRetrier = ctx.transactionRetrier;

      const retryError = new Error("error for testing");
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(new Message(testPendingMessage));
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(transactionRetrier, "retryWithHigherFee").mockRejectedValue(retryError);

      await messageClaimingPersister.process();

      expect(databaseService.updateMessage).not.toHaveBeenCalled();
    });

    it("Should cancel and reset message when max bumps per cycle exceeded", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimNumberOfRetry: 5,
        claimCycleCount: 0,
        claimTxNonce: 42,
      });
      const ctx = createPersister({ messageSubmissionTimeout: 0, maxBumpsPerCycle: 5, maxCycles: 2 });
      messageClaimingPersister = ctx.persister;
      provider = ctx.provider;
      databaseService = ctx.databaseService;
      messageServiceContract = ctx.messageServiceContract;
      transactionRetrier = ctx.transactionRetrier;

      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(transactionRetrier, "cancelTransaction").mockResolvedValue("0xcancelhash" as `0x${string}`);
      jest.spyOn(receiptPoller, "poll").mockResolvedValue(generateReceipt());
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      // testPendingMessage has no claimTxMaxFeePerGas/claimTxMaxPriorityFeePerGas, so stuckFees is undefined
      expect(transactionRetrier.cancelTransaction).toHaveBeenCalledWith(42, undefined);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          status: MessageStatus.SENT,
          claimNumberOfRetry: 0,
          claimCycleCount: 1,
        }),
      );
    });

    it("Should set NEEDS_MANUAL_INTERVENTION when max cycles exceeded", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimNumberOfRetry: 5,
        claimCycleCount: 2,
      });
      const ctx = createPersister({ messageSubmissionTimeout: 0, maxBumpsPerCycle: 5, maxCycles: 2 });
      messageClaimingPersister = ctx.persister;
      provider = ctx.provider;
      databaseService = ctx.databaseService;
      logger = ctx.logger;

      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.NEEDS_MANUAL_INTERVENTION }),
      );
      expect(loggerErrorSpy).toHaveBeenCalledWith(
        "Max retry cycles exceeded. Manual intervention is needed.",
        expect.any(Object),
      );
    });

    it("Should reset pending message without claim tx hash to SENT", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimTxHash: undefined,
      });
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(expect.objectContaining({ status: MessageStatus.SENT }));
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Found pending message without claim tx hash, resetting to allow retry.",
        expect.any(Object),
      );
    });

    it("Should update receipt status with processing time metrics for L1_TO_L2 direction", async () => {
      const creationDate = new Date(2024, 1, 1, 0, 0, 0);
      const blockTimestamp = creationDate.getTime() / 1_000 + 60;
      const pendingMsgWithCreationDate = new Message({
        ...testPendingMessage,
        claimTxCreationDate: creationDate,
        direction: Direction.L1_TO_L2,
      });
      const txReceipt = generateTransactionReceipt({ status: "success", blockNumber: 42 });

      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsgWithCreationDate);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      jest.spyOn(provider, "getBlock").mockResolvedValue({ number: 42, timestamp: blockTimestamp } as never);
      jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();
      const addProcessingTimeSpy = jest.spyOn(transactionMetricsUpdater, "addTransactionProcessingTime");
      const addConfirmationTimeSpy = jest.spyOn(transactionMetricsUpdater, "addTransactionInfuraConfirmationTime");

      await messageClaimingPersister.process();

      expect(addProcessingTimeSpy).toHaveBeenCalledWith(Direction.L1_TO_L2, expect.any(Number));
      expect(addConfirmationTimeSpy).toHaveBeenCalledWith(Direction.L1_TO_L2, expect.any(Number));
    });

    it("Should increment sponsorship fee when message isForSponsorship", async () => {
      const pendingMsgForSponsorship = new Message({
        ...testPendingMessage,
        isForSponsorship: true,
      });
      const txReceipt = generateTransactionReceipt({ status: "success", gasUsed: 100_000n, gasPrice: 1_000_000n });

      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsgForSponsorship);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      const sponsorshipSpy = jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();

      await messageClaimingPersister.process();

      expect(sponsorshipSpy).toHaveBeenCalledWith(100_000n * 1_000_000n, Direction.L1_TO_L2);
    });

    it("Should handle cancelAndResetMessage when message is CLAIMED on-chain and receipt is found", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimNumberOfRetry: 5,
        claimCycleCount: 0,
        claimTxNonce: 42,
      });
      const ctx = createPersister({ messageSubmissionTimeout: 0, maxBumpsPerCycle: 5, maxCycles: 2 });
      messageClaimingPersister = ctx.persister;
      provider = ctx.provider;
      databaseService = ctx.databaseService;
      messageServiceContract = ctx.messageServiceContract;
      sponsorshipMetricsUpdater = ctx.sponsorshipMetricsUpdater;

      const txReceipt = generateTransactionReceipt({ status: "success" });
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValueOnce(null).mockResolvedValueOnce(txReceipt);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.CLAIMED_SUCCESS }),
      );
    });

    it("Should log warning when cancelAndResetMessage finds CLAIMED message but no receipt", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimNumberOfRetry: 5,
        claimCycleCount: 0,
        claimTxNonce: 42,
      });
      const ctx = createPersister({ messageSubmissionTimeout: 0, maxBumpsPerCycle: 5, maxCycles: 2 });
      messageClaimingPersister = ctx.persister;
      provider = ctx.provider;
      databaseService = ctx.databaseService;
      messageServiceContract = ctx.messageServiceContract;

      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).not.toHaveBeenCalled();
    });

    it("Should skip cancelTransaction when claimTxNonce is undefined", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimNumberOfRetry: 5,
        claimCycleCount: 0,
      });
      const ctx = createPersister({ messageSubmissionTimeout: 0, maxBumpsPerCycle: 5, maxCycles: 2 });
      messageClaimingPersister = ctx.persister;
      provider = ctx.provider;
      databaseService = ctx.databaseService;
      messageServiceContract = ctx.messageServiceContract;

      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(ctx.transactionRetrier.cancelTransaction).not.toHaveBeenCalled();
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(expect.objectContaining({ status: MessageStatus.SENT }));
    });

    it("Should log error when cancelAndResetMessage throws", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimNumberOfRetry: 5,
        claimCycleCount: 0,
        claimTxNonce: 42,
      });
      const ctx = createPersister({ messageSubmissionTimeout: 0, maxBumpsPerCycle: 5, maxCycles: 2 });
      messageClaimingPersister = ctx.persister;
      provider = ctx.provider;
      databaseService = ctx.databaseService;
      messageServiceContract = ctx.messageServiceContract;

      const cancelError = new Error("cancel failed");
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      jest.spyOn(messageServiceContract, "getMessageStatus").mockRejectedValue(cancelError);

      await messageClaimingPersister.process();

      expect(databaseService.updateMessage).not.toHaveBeenCalled();
    });

    it("Should return early when receipt is null and message has not exceeded submission timeout", async () => {
      const recentDate = new Date();
      const pendingMsg = new Message({
        ...testPendingMessage,
        updatedAt: recentDate,
      });
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      const getMessageStatusSpy = jest.spyOn(messageServiceContract, "getMessageStatus");

      await messageClaimingPersister.process();

      expect(getMessageStatusSpy).not.toHaveBeenCalled();
    });

    it("Should log error without messageHash when getFirstPendingMessage throws", async () => {
      const dbError = new Error("database connection lost");
      jest.spyOn(databaseService, "getFirstPendingMessage").mockRejectedValue(dbError);
      const loggerErrorSpy = jest.spyOn(logger, "error");

      await messageClaimingPersister.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error processing pending message.", {
        error: dbError,
      });
    });

    it("Should log reverted message without processing time when claimTxCreationDate is not set", async () => {
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimTxCreationDate: undefined,
      });
      const txReceipt = generateTransactionReceipt({ status: "reverted" });
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      jest.spyOn(messageServiceContract, "isRateLimitExceededError").mockResolvedValue(false);
      const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await messageClaimingPersister.process();

      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.CLAIMED_REVERTED }),
      );
    });

    it("Should log reverted message WITH processing time when claimTxCreationDate is set", async () => {
      const creationDate = new Date(Date.now() - 5000);
      const pendingMsg = new Message({
        ...testPendingMessage,
        claimTxCreationDate: creationDate,
      });
      const txReceipt = generateTransactionReceipt({ status: "reverted" });
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(pendingMsg);
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      jest.spyOn(provider, "getBlock").mockResolvedValue({
        number: txReceipt.blockNumber,
        timestamp: Math.floor(Date.now() / 1000),
        hash: "0x0000000000000000000000000000000000000000000000000000000000000001",
      });
      jest.spyOn(messageServiceContract, "isRateLimitExceededError").mockResolvedValue(false);
      const addProcessingTimeSpy = jest.spyOn(transactionMetricsUpdater, "addTransactionProcessingTime");

      await messageClaimingPersister.process();

      expect(addProcessingTimeSpy).toHaveBeenCalledWith(Direction.L1_TO_L2, expect.any(Number));
    });

    it("Should handle two consecutive process calls correctly", async () => {
      const retryTxReceipt = generateTransactionReceipt({ status: "success" });
      const retryTxResponse = generateTransactionSubmission({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const testPendingMessageLocal = new Message(testPendingMessage);
      const testPendingMessageLocal2 = new Message(testPendingMessage2);

      jest
        .spyOn(databaseService, "getFirstPendingMessage")
        .mockResolvedValueOnce(testPendingMessageLocal)
        .mockResolvedValueOnce(testPendingMessageLocal2);
      jest
        .spyOn(provider, "getTransactionReceipt")
        .mockResolvedValueOnce(null)
        .mockResolvedValueOnce(null)
        .mockResolvedValueOnce(null)
        .mockResolvedValueOnce(null);
      jest
        .spyOn(messageServiceContract, "getMessageStatus")
        .mockResolvedValueOnce(OnChainMessageStatus.CLAIMABLE)
        .mockResolvedValueOnce(OnChainMessageStatus.CLAIMED);
      jest.spyOn(transactionRetrier, "retryWithHigherFee").mockResolvedValue(retryTxResponse);
      jest.spyOn(receiptPoller, "poll").mockResolvedValue(retryTxReceipt);
      jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();

      await messageClaimingPersister.process();
      await messageClaimingPersister.process();

      expect(databaseService.updateMessage).toHaveBeenCalled();
    });
  });
});
