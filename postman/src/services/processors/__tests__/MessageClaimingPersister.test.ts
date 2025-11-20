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
import { ISponsorshipMetricsUpdater, ITransactionMetricsUpdater } from "../../../../src/core/metrics";

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
  const sponsorshipMetricsUpdater = mock<ISponsorshipMetricsUpdater>();
  const transactionMetricsUpdater = mock<ITransactionMetricsUpdater>();
  const provider =
    mock<IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, JsonRpcProvider>>();
  const logger = new TestLogger(MessageClaimingPersister.name);

  beforeEach(() => {
    messageClaimingPersister = new MessageClaimingPersister(
      databaseService,
      l2MessageServiceContractMock,
      sponsorshipMetricsUpdater,
      transactionMetricsUpdater,
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

  type TestFixtureFactoryOpts = {
    firstPendingMessage?: Message | null;
    secondPendingMessage?: Message | null;
    txReceipt?: TransactionReceipt | null;
    txReceiptError?: Error;
    retryTxReceipt?: TransactionReceipt | null;
    isRateLimitExceededError?: boolean;
    firstOnChainMessageStatus?: OnChainMessageStatus;
    secondOnChainMessageStatus?: OnChainMessageStatus;
    retryTransactionWithHigherFeeResponse?: TransactionResponse;
    retryTransactionWithHigherFeeError?: Error;
    retryTransactionWithHigherFeeReceipt?: TransactionReceipt | null;
  };

  type TestFixture = {
    l2QuerierGetReceiptSpy: jest.SpyInstance<ReturnType<typeof provider.getTransactionReceipt>>;
    loggerErrorSpy: jest.SpyInstance<ReturnType<typeof logger.error>>;
    loggerWarnSpy: jest.SpyInstance<ReturnType<typeof logger.warn>>;
    loggerInfoSpy: jest.SpyInstance<ReturnType<typeof logger.info>>;
    messageRepositoryUpdateSpy: jest.SpyInstance<ReturnType<typeof databaseService.updateMessage>>;
  };

  const testFixtureFactory = (opts: TestFixtureFactoryOpts): TestFixture => {
    const {
      firstPendingMessage,
      secondPendingMessage,
      txReceipt,
      txReceiptError,
      retryTxReceipt,
      isRateLimitExceededError,
      firstOnChainMessageStatus,
      secondOnChainMessageStatus,
      retryTransactionWithHigherFeeResponse,
      retryTransactionWithHigherFeeError,
      retryTransactionWithHigherFeeReceipt,
    } = opts;
    if (firstPendingMessage !== undefined && secondPendingMessage !== undefined) {
      jest
        .spyOn(databaseService, "getFirstPendingMessage")
        .mockResolvedValueOnce(firstPendingMessage)
        .mockResolvedValueOnce(secondPendingMessage);
    } else if (firstPendingMessage !== undefined) {
      jest.spyOn(databaseService, "getFirstPendingMessage").mockResolvedValue(firstPendingMessage);
    }
    if (txReceiptError !== undefined) {
      jest.spyOn(provider, "getTransactionReceipt").mockRejectedValue(txReceiptError);
    } else if (txReceipt !== undefined) {
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(txReceipt);
    }
    if (isRateLimitExceededError !== undefined)
      jest.spyOn(l2MessageServiceContractMock, "isRateLimitExceededError").mockResolvedValue(isRateLimitExceededError);
    if (retryTxReceipt !== undefined)
      jest.spyOn(provider, "getTransactionReceipt").mockResolvedValueOnce(null).mockResolvedValueOnce(retryTxReceipt);
    if (firstOnChainMessageStatus !== undefined && secondOnChainMessageStatus !== undefined) {
      jest
        .spyOn(l2MessageServiceContractMock, "getMessageStatus")
        .mockResolvedValueOnce(firstOnChainMessageStatus)
        .mockResolvedValueOnce(secondOnChainMessageStatus);
    } else if (firstOnChainMessageStatus !== undefined) {
      jest.spyOn(l2MessageServiceContractMock, "getMessageStatus").mockResolvedValue(firstOnChainMessageStatus);
    }
    if (retryTransactionWithHigherFeeError !== undefined) {
      jest
        .spyOn(l2MessageServiceContractMock, "retryTransactionWithHigherFee")
        .mockRejectedValue(retryTransactionWithHigherFeeError);
    } else if (
      retryTransactionWithHigherFeeResponse !== undefined &&
      retryTransactionWithHigherFeeReceipt !== undefined
    ) {
      jest
        .spyOn(l2MessageServiceContractMock, "retryTransactionWithHigherFee")
        .mockResolvedValue(retryTransactionWithHigherFeeResponse);
      jest.spyOn(retryTransactionWithHigherFeeResponse, "wait").mockResolvedValue(retryTransactionWithHigherFeeReceipt);
    }

    jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
    jest.spyOn(sponsorshipMetricsUpdater, "incrementSponsorshipFeePaid").mockResolvedValue();

    const l2QuerierGetReceiptSpy = jest.spyOn(provider, "getTransactionReceipt");
    const loggerErrorSpy = jest.spyOn(logger, "error");
    const loggerWarnSpy = jest.spyOn(logger, "warn");
    const loggerInfoSpy = jest.spyOn(logger, "info");
    const messageRepositoryUpdateSpy = jest.spyOn(databaseService, "updateMessage");

    return {
      l2QuerierGetReceiptSpy,
      loggerErrorSpy,
      loggerWarnSpy,
      loggerInfoSpy,
      messageRepositoryUpdateSpy,
    };
  };

  describe("process", () => {
    it("Should early return immediately if no pending message found", async () => {
      const { l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: null,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as error if getTransactionReceipt throws error", async () => {
      const getTxReceiptError = new Error("error for testing");
      const testPendingMessageLocal = new Message(testPendingMessage);
      const { loggerErrorSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        txReceiptError: getTxReceiptError,
      });

      await messageClaimingPersister.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error processing message.", {
        messageHash: testPendingMessage.messageHash,
        errorCode: "UNKNOWN_ERROR",
        errorMessage: getTxReceiptError.message,
      });
    });

    it("Should log as info and update message as claimed success if successful", async () => {
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const testPendingMessageLocal = new Message(testPendingMessage);
      const expectedSavedMessage = new Message({
        ...testPendingMessageLocal,
        status: MessageStatus.CLAIMED_SUCCESS,
        updatedAt: mockedDate,
      });
      const { loggerInfoSpy, messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: new Message(testPendingMessage),
        txReceipt: txReceipt,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(expectedSavedMessage);
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith(
        "Message has been SUCCESSFULLY claimed: messageHash=%s transactionHash=%s",
        expectedSavedMessage.messageHash,
        expectedSavedMessage.claimTxHash,
        {},
      );
    });

    it("Should return and update message as sent if receipt status is 0 and rate limit exceeded", async () => {
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 0 });
      const testPendingMessageLocal = new Message(testPendingMessage);
      const expectedSavedMessage = new Message({
        ...testPendingMessage,
        status: MessageStatus.SENT,
        updatedAt: mockedDate,
      });
      const { messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        txReceipt: txReceipt,
        isRateLimitExceededError: true,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as warning and update message as claim reverted if receipt status is 0", async () => {
      const txReceipt = testingHelpers.generateTransactionReceipt({ status: 0 });
      const testPendingMessageLocal = new Message(testPendingMessage);
      const expectedSavedMessage = new Message({
        ...testPendingMessage,
        status: MessageStatus.CLAIMED_REVERTED,
        updatedAt: mockedDate,
      });
      const { loggerWarnSpy, l2QuerierGetReceiptSpy, messageRepositoryUpdateSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        txReceipt,
        isRateLimitExceededError: false,
      });
      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(expectedSavedMessage);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Message claim transaction has been REVERTED: messageHash=%s transactionHash=%s",
        expectedSavedMessage.messageHash,
        expectedSavedMessage.claimTxHash,
        {},
      );
    });

    it("Should update message as claimed if retry receipt successful and message claimed on-chain", async () => {
      const retryTxReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const testPendingMessageLocal = new Message(testPendingMessage);
      const expectedSavedMessage = new Message({
        ...testPendingMessageLocal,
        status: MessageStatus.CLAIMED_SUCCESS,
        updatedAt: mockedDate,
      });
      const { loggerWarnSpy, messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: new Message(testPendingMessageLocal),
        retryTxReceipt,
        isRateLimitExceededError: false,
        firstOnChainMessageStatus: OnChainMessageStatus.CLAIMED,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(expectedSavedMessage);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(2);
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        1,
        "Retrying to claim message: messageHash=%s",
        testPendingMessage.messageHash,
      );
      expect(loggerWarnSpy).toHaveBeenNthCalledWith(
        2,
        "Retried claim message transaction succeed: messageHash=%s transactionHash=%s",
        testPendingMessage.messageHash,
        retryTxReceipt.hash,
      );
    });

    it("Should return and log as warning if message is claimed but receipt returned as null", async () => {
      const testPendingMessageLocal = new Message(testPendingMessage);
      const { loggerWarnSpy, messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        txReceipt: null,
        isRateLimitExceededError: false,
        firstOnChainMessageStatus: OnChainMessageStatus.CLAIMED,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(0);
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
      const retryTxReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const retryTxResponse = testingHelpers.generateTransactionResponse({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const testPendingMessageLocal = new Message(testPendingMessage);
      const expectedSavedMessage = new Message({
        ...testPendingMessageLocal,
        status: MessageStatus.CLAIMED_SUCCESS,
        updatedAt: mockedDate,
        claimTxNonce: retryTxResponse.nonce,
        claimTxGasLimit: Number(retryTxResponse.gasLimit),
        claimNumberOfRetry: 1,
        claimLastRetriedAt: mockedDate,
        claimTxCreationDate: expect.any(Date),
      });
      const { loggerWarnSpy, messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        txReceipt: null,
        isRateLimitExceededError: false,
        firstOnChainMessageStatus: OnChainMessageStatus.CLAIMABLE,
        retryTransactionWithHigherFeeResponse: retryTxResponse,
        retryTransactionWithHigherFeeReceipt: retryTxReceipt,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositoryUpdateSpy).toHaveBeenNthCalledWith(1, testPendingMessageLocal);
      expect(messageRepositoryUpdateSpy).toHaveBeenNthCalledWith(2, expectedSavedMessage);

      // expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(testPendingMessageLocal);
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
        retryTxReceipt.hash,
      );
    });

    it("Should update DB successfully if first process claimable message with receipt, then process claimed message with no receipt", async () => {
      const retryTxReceipt = testingHelpers.generateTransactionReceipt({ status: 1 });
      const retryTxResponse = testingHelpers.generateTransactionResponse({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const testPendingMessageLocal = new Message(testPendingMessage);
      const testPendingMessageLocal2 = new Message(testPendingMessage2);
      const expectedSavedMessage = new Message({
        ...testPendingMessageLocal,
        status: MessageStatus.CLAIMED_SUCCESS,
        updatedAt: mockedDate,
        claimTxNonce: retryTxResponse.nonce,
        claimTxGasLimit: Number(retryTxResponse.gasLimit),
        claimNumberOfRetry: 1,
        claimLastRetriedAt: mockedDate,
        claimTxCreationDate: expect.any(Date),
      });
      const { loggerWarnSpy, messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        secondPendingMessage: testPendingMessageLocal2,
        txReceipt: null,
        isRateLimitExceededError: false,
        firstOnChainMessageStatus: OnChainMessageStatus.CLAIMABLE,
        secondOnChainMessageStatus: OnChainMessageStatus.CLAIMED,
        retryTransactionWithHigherFeeResponse: retryTxResponse,
        retryTransactionWithHigherFeeReceipt: retryTxReceipt,
      });

      await messageClaimingPersister.process();
      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(3);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositoryUpdateSpy).toHaveBeenNthCalledWith(1, testPendingMessageLocal);
      expect(messageRepositoryUpdateSpy).toHaveBeenNthCalledWith(2, expectedSavedMessage);
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
        retryTxReceipt.hash,
      );
    });

    it("Should return and log as warning if message is claimable but retry tx throws error", async () => {
      messageClaimingPersister = new MessageClaimingPersister(
        databaseService,
        l2MessageServiceContractMock,
        sponsorshipMetricsUpdater,
        transactionMetricsUpdater,
        provider,
        {
          direction: Direction.L1_TO_L2,
          messageSubmissionTimeout: 0,
          maxTxRetries: 0,
        },
        logger,
      );
      const testPendingMessageLocal = new Message(testPendingMessage);
      const retryError = new Error("error for testing");
      const { loggerWarnSpy, loggerErrorSpy, messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        txReceipt: null,
        isRateLimitExceededError: false,
        firstOnChainMessageStatus: OnChainMessageStatus.CLAIMABLE,
        retryTransactionWithHigherFeeError: retryError,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(0);
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

    it("Should return and log as error if retry tx fails to get receipt", async () => {
      const retryTxResponse = testingHelpers.generateTransactionResponse();
      const testPendingMessageLocal = new Message(testPendingMessage);
      jest.spyOn(l2MessageServiceContractMock, "retryTransactionWithHigherFee").mockResolvedValue(retryTxResponse);
      const { loggerWarnSpy, loggerErrorSpy, messageRepositoryUpdateSpy, l2QuerierGetReceiptSpy } = testFixtureFactory({
        firstPendingMessage: testPendingMessageLocal,
        txReceipt: null,
        isRateLimitExceededError: false,
        firstOnChainMessageStatus: OnChainMessageStatus.CLAIMABLE,
        retryTransactionWithHigherFeeResponse: retryTxResponse,
        retryTransactionWithHigherFeeReceipt: null,
      });

      await messageClaimingPersister.process();

      expect(l2QuerierGetReceiptSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryUpdateSpy).toHaveBeenCalledWith(testPendingMessageLocal);
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
