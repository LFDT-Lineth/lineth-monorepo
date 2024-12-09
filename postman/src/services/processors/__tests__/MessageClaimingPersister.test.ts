import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { Direction, OnChainMessageStatus, testingHelpers } from "@consensys/linea-sdk";
import { TestLogger } from "../../../utils/testing/helpers";
import { MessageStatus } from "../../../core/enums";
import { testL2NetworkConfig, testPendingMessage, testPendingMessage2 } from "../../../utils/testing/constants";
import { IMessageServiceContract } from "../../../core/services/contracts/IMessageServiceContract";
import {
  Block,
  ContractTransactionResponse,
  ErrorDescription,
  JsonRpcProvider,
  Overrides,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { IGasProvider } from "../../../core/clients/blockchain/IGasProvider";
import { Message } from "../../../core/entities/Message";
import { IMessageClaimingPersister } from "../../../core/services/processors/IMessageClaimingPersister";
import { MessageClaimingPersister } from "../MessageClaimingPersister";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { IProvider } from "../../../core/clients/blockchain/IProvider";

describe("TestMessageClaimingPersister ", () => {
  let messageClaimingPersister: IMessageClaimingPersister;
  let mockedDate: Date;
  const databaseService = mock<EthereumMessageDBService>();
  const l2MessageServiceContractMock = mock<
    IMessageServiceContract<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      ErrorDescription
    > &
      IGasProvider<TransactionRequest>
  >();
  const provider =
    mock<IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, JsonRpcProvider>>();
  const logger = new TestLogger(MessageClaimingPersister.name);

  beforeEach(() => {
    messageClaimingPersister = new MessageClaimingPersister(
      databaseService,
      l2MessageServiceContractMock,
      provider,
      {
        direction: Direction.L1_TO_L2,
        messageSubmissionTimeout: testL2NetworkConfig.claiming.messageSubmissionTimeout,
        maxTxRetries: testL2NetworkConfig.claiming.maxTxRetries,
      },
      logger,
    );

    mockedDate = new Date();
    jest.useFakeTimers();
    jest.setSystemTime(mockedDate);
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("process", () => {
    it("Should return if getTransactionReceipt return null", async () => {
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt");
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(null);

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as error if getTransactionReceipt throws error", async () => {
      const testPendingMessageLocal = new Message(testPendingMessage);
      const getTxReceiptError = new Error("error for testing");
      const loggerErrorSpy = jest.spyOn(logger, "error");
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getTransactionReceipt").mockRejectedValue(getTxReceiptError);

      await messageClaimingPersister.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith(getTxReceiptError);
    });

    it("Should log as info and update message as claimed success if successful", async () => {
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const loggerInfoSpy = jest.spyOn(logger, "info");
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      const expectedSavedMessage = new Message({
        ...testPendingMessageLocal,
        status: MessageStatus.CLAIMED_SUCCESS,
        updatedAt: mockedDate,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith(
        "Message has been SUCCESSFULLY claimed: messageHash=%s transactionHash=%s",
        expectedSavedMessage.messageHash,
        expectedSavedMessage.claimTxHash,
      );
    });

    it("Should return and update message as sent if receipt status is 0 and rate limit exceeded", async () => {
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 0 });
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(true);
      const expectedSavedMessage = new Message({
        ...testPendingMessageLocal,
        status: MessageStatus.SENT,
        updatedAt: mockedDate,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as warning and update message as claimed reverted if receipt status is 0", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 0 });
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(false);
      const expectedSavedMessage = new Message({
        ...testPendingMessageLocal,
        status: MessageStatus.CLAIMED_REVERTED,
        updatedAt: mockedDate,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Message claim transaction has been REVERTED: messageHash=%s transactionHash=%s",
        expectedSavedMessage.messageHash,
        expectedSavedMessage.claimTxHash,
      );
    });

    it("Should return and log as warning if message is claimed and receipt was retrieved successfully", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const l2QuerierGetReceiptSpy = jest
        .spyOn(provider, "getTransactionReceipt")
        .mockResolvedValueOnce(null)
        .mockResolvedValueOnce(txReceipt);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "saveMessages");
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(false);
      jest.spyOn(l2MessageServiceContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(0);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(2);
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        1,
        "Retrying to claim message: messageHash=%s",
        testPendingMessageLocal.messageHash,
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        2,
        "Retried claim message transaction succeed: messageHash=%s transactionHash=%s",
        testPendingMessageLocal.messageHash,
        txReceipt.hash,
      );
    });

    it("Should return and log as warning if message is claimed but receipt returned as null", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "saveMessages");
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(false);
      jest.spyOn(l2MessageServiceContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(0);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(2);
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        1,
        "Retrying to claim message: messageHash=%s",
        testPendingMessageLocal.messageHash,
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        2,
        "Calling retryTransaction again as message was claimed but transaction receipt is not available yet: messageHash=%s transactionHash=%s",
        testPendingMessageLocal.messageHash,
        testPendingMessageLocal.claimTxHash,
      );
    });

    it("Should return and log as warning if message is claimable and retry tx was sent successfully", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const txResponse = testingHelpers.generateTransactionResponse({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const testPendingMessageLocal = new Message(testPendingMessage);

      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(false);
      jest
        .spyOn(l2MessageServiceContractMock, "getMessageStatus")
        .mockResolvedValueOnce(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(l2MessageServiceContractMock, "retryTransactionWithHigherFee").mockResolvedValue(txResponse);
      jest.spyOn(txResponse, "wait").mockResolvedValue(txReceipt);

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(testPendingMessageLocal);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(3);
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        1,
        "Retrying to claim message: messageHash=%s",
        testPendingMessage.messageHash,
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        2,
        "Retry to claim message: numberOfRetries=%s messageInfo=%s",
        "1",
        testPendingMessage.toString(),
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        3,
        "Retried claim message transaction succeed: messageHash=%s transactionHash=%s",
        testPendingMessageLocal.messageHash,
        txReceipt.hash,
      );
    });

    it("Should return and log as warning if message is claimable and retry tx was sent successfully", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const txResponse = testingHelpers.generateTransactionResponse({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const testPendingMessageLocal = new Message(testPendingMessage);
      const testPendingMessageLocal2 = new Message(testPendingMessage2);
      jest
        .spyOn(databaseService, "getFirstPendingMessage")
        .mockResolvedValueOnce(testPendingMessageLocal)
        .mockResolvedValueOnce(testPendingMessageLocal2);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(false);
      jest
        .spyOn(l2MessageServiceContractMock, "getMessageStatus")
        .mockResolvedValueOnce(OnChainMessageStatus.CLAIMABLE)
        .mockResolvedValueOnce(OnChainMessageStatus.CLAIMED);
      jest.spyOn(l2MessageServiceContractMock, "retryTransactionWithHigherFee").mockResolvedValue(txResponse);
      jest.spyOn(txResponse, "wait").mockResolvedValue(txReceipt);

      await messageClaimingPersister.process();
      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(3);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(testPendingMessageLocal);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(5);
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        1,
        "Retrying to claim message: messageHash=%s",
        testPendingMessage.messageHash,
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        2,
        "Retry to claim message: numberOfRetries=%s messageInfo=%s",
        "1",
        testPendingMessage.toString(),
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        3,
        "Retried claim message transaction succeed: messageHash=%s transactionHash=%s",
        testPendingMessageLocal.messageHash,
        txReceipt.hash,
      );
    });

    it("Should return and log as warning if message is claimable but retry tx throws error", async () => {
      messageClaimingPersister = new MessageClaimingPersister(
        databaseService,
        l2MessageServiceContractMock,
        provider,
        {
          direction: Direction.L1_TO_L2,
          messageSubmissionTimeout: 0,
          maxTxRetries: 0,
        },
        logger,
      );
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "saveMessages");
      const testPendingMessageLocal = new Message(testPendingMessage);
      const retryError = new Error("error for testing");
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(false);
      jest.spyOn(l2MessageServiceContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(l2MessageServiceContractMock, "retryTransactionWithHigherFee").mockRejectedValue(retryError);

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(0);
      expect(loggerErrorSpy).toHaveBeenCalledTimes(2);
      expect(loggerErrorSpy).toHaveBeenNthCalledWith(
        1,
        "Transaction retry failed: messageHash=%s error=%s",
        testPendingMessage.messageHash,
        retryError,
      );
      expect(loggerErrorSpy).toHaveBeenNthCalledWith(
        2,
        `Max number of retries exceeded. Manual intervention is needed as soon as possible: messageInfo=%s`,
        testPendingMessage.toString(),
      );
      expect(loggerWarnSpy).toHaveBeenCalledTimes(2);
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        1,
        "Retrying to claim message: messageHash=%s",
        testPendingMessage.messageHash,
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        2,
        "Retry to claim message: numberOfRetries=%s messageInfo=%s",
        "1",
        testPendingMessage.toString(),
      );
    });

    it("Should return and log as error if message is claimable but tx response wait returns null", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const txResponse = testingHelpers.generateTransactionResponse();
      const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "saveMessages");
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(testPendingMessageLocal);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(false);
      jest.spyOn(l2MessageServiceContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(l2MessageServiceContractMock, "retryTransactionWithHigherFee").mockResolvedValue(txResponse);
      jest.spyOn(txResponse, "wait").mockResolvedValue(null);

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(0);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(2);
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        1,
        "Retrying to claim message: messageHash=%s",
        testPendingMessage.messageHash,
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        2,
        "Retry to claim message: numberOfRetries=%s messageInfo=%s",
        "1",
        testPendingMessage.toString(),
      );
      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
    });
  });
});
