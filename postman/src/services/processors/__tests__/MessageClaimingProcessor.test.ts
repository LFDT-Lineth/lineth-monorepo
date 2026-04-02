import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { ILineaRollupClient } from "../../../core/clients/blockchain/ethereum/ILineaRollupClient";
import { IEthereumGasProvider } from "../../../core/clients/blockchain/IGasProvider";
import {
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_NUMBER_OF_RETRIES,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_PROFIT_MARGIN,
  DEFAULT_RETRY_DELAY_IN_SECONDS,
} from "../../../core/constants";
import { Message } from "../../../core/entities/Message";
import { Direction, MessageStatus, OnChainMessageStatus } from "../../../core/enums";
import { IErrorParser } from "../../../core/errors/IErrorParser";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { INonceManager } from "../../../core/services/INonceManager";
import { IMessageClaimingProcessor } from "../../../core/services/processors/IMessageClaimingProcessor";
import { ViemErrorParser } from "../../../infrastructure/blockchain/viem";
import {
  DEFAULT_MAX_FEE_PER_GAS,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_2,
  TEST_TRANSACTION_HASH,
  testAnchoredMessage,
  testClaimedMessage,
  testL2NetworkConfig,
  testUnderpricedAnchoredMessage,
  testZeroFeeAnchoredMessage,
} from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { EthereumTransactionValidationService } from "../../EthereumTransactionValidationService";
import { MessageClaimingProcessor } from "../MessageClaimingProcessor";

describe("TestMessageClaimingProcessor", () => {
  let messageClaimingProcessor: IMessageClaimingProcessor;
  let gasProvider: IEthereumGasProvider;
  let messageRepository: ReturnType<typeof mock<IMessageRepository>>;
  let getNextMessageToClaim: jest.Mock<Promise<Message | null>, []>;
  let transactionValidationService: EthereumTransactionValidationService;
  let mockedDate: Date;
  const lineaRollupContractMock = mock<ILineaRollupClient>();
  const nonceManager = mock<INonceManager>();
  const errorParser = new ViemErrorParser();

  const logger = new TestLogger(MessageClaimingProcessor.name);

  beforeEach(() => {
    gasProvider = mock<IEthereumGasProvider>();
    messageRepository = mock<IMessageRepository>();
    getNextMessageToClaim = jest.fn();
    transactionValidationService = new EthereumTransactionValidationService(
      lineaRollupContractMock,
      gasProvider,
      {
        profitMargin: DEFAULT_PROFIT_MARGIN,
        maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
        isPostmanSponsorshipEnabled: DEFAULT_ENABLE_POSTMAN_SPONSORING,
        maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
      },
      logger,
    );
    messageClaimingProcessor = new MessageClaimingProcessor(
      lineaRollupContractMock,
      nonceManager,
      messageRepository,
      getNextMessageToClaim,
      transactionValidationService,
      errorParser,
      {
        profitMargin: DEFAULT_PROFIT_MARGIN,
        maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
        retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
        maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
        direction: Direction.L2_TO_L1,
        originContractAddress: TEST_CONTRACT_ADDRESS_2,
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
    it("Should return without calling any get message status if getFirstMessageToClaim return null", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      getNextMessageToClaim.mockResolvedValue(null);

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as warning and save message as zero fee if message has zero fee", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testZeroFeeAnchoredMessage);
      jest.spyOn(transactionValidationService, "evaluateTransaction").mockResolvedValueOnce({
        hasZeroFee: true,
        isRateLimitExceeded: false,
        isUnderPriced: false,
        isForSponsorship: false,
        estimatedGasLimit: 50_000n,
        threshold: 5,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });

      const expectedLoggingMessage = new Message(testZeroFeeAnchoredMessage);
      const expectedSavedMessage = new Message({
        ...testZeroFeeAnchoredMessage,
        status: MessageStatus.ZERO_FEE,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("Found message with zero fee. This message will not be processed.", {
        messageHash: expectedLoggingMessage.messageHash,
      });
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as info and save message as claimed if message was claimed", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testClaimedMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      const expectedLoggingMessage = new Message(testClaimedMessage);
      const expectedSavedMessage = new Message({ ...testClaimedMessage, updatedAt: mockedDate });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledTimes(2);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Found already claimed message.", {
        messageHash: expectedLoggingMessage.messageHash,
      });
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as warning and save message as non-executable if message gas limit was above max gas limit", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(DEFAULT_MAX_CLAIM_GAS_LIMIT * 2n);
      const expectedLoggingMessage = new Message(testAnchoredMessage);
      const expectedSavedMessage = new Message({
        ...testAnchoredMessage,
        status: MessageStatus.NON_EXECUTABLE,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Estimated gas limit is higher than the max allowed gas limit for this message.",
        {
          messageHash: expectedLoggingMessage.messageHash,
          messageInfo: expectedLoggingMessage.toString(),
          estimatedGasLimit: undefined,
          maxAllowedGasLimit: testL2NetworkConfig.claiming.maxClaimGasLimit!.toString(),
        },
      );
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as warning and save message as fee underpriced if message fee was underpriced", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testUnderpricedAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      const expectedLoggingMessage = new Message({
        ...testUnderpricedAnchoredMessage,
        claimGasEstimationThreshold: 10,
        updatedAt: mockedDate,
      });
      const expectedSavedMessage = new Message({
        ...testUnderpricedAnchoredMessage,
        claimGasEstimationThreshold: 10,
        status: MessageStatus.FEE_UNDERPRICED,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("Fee underpriced found in this message.", {
        messageHash: expectedLoggingMessage.messageHash,
        messageInfo: expectedLoggingMessage.toString(),
        transactionGasLimit: "100000",
        maxFeePerGas: "1000000000",
      });
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenNthCalledWith(2, expectedSavedMessage);
    });

    it("Should log as warning and save message with reset claimGasEstimationThreshold if rate limit exceeded on L1", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(true);
      const expectedLoggingMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedLoggingMessage);
      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Rate limit exceeded for this message. It will be reprocessed later.",
        { messageHash: expectedLoggingMessage.messageHash },
      );
    });

    it("Should update message if successful", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      jest.spyOn(lineaRollupContractMock, "claim").mockResolvedValue({
        hash: TEST_TRANSACTION_HASH,
        nonce: 101,
        gasLimit: 100_000n,
        maxFeePerGas: 1000000000n,
        maxPriorityFeePerGas: 1000000000n,
      });
      const expectedFinalMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        isForSponsorship: false,
        claimTxNonce: 101,
        status: MessageStatus.PENDING,
        claimTxCreationDate: mockedDate,
        claimTxGasLimit: 100000,
        claimTxMaxFeePerGas: 1000000000n,
        claimTxMaxPriorityFeePerGas: 1000000000n,
        claimTxHash: TEST_TRANSACTION_HASH,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedFinalMessage);
      expect(nonceManager.commitNonce).toHaveBeenCalledWith(101);
    });

    it("Should rollback nonce without touching DB when chain claim() call fails", async () => {
      const localAnchoredMessage = new Message(testAnchoredMessage);
      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(localAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      jest.spyOn(lineaRollupContractMock, "claim").mockRejectedValue(new Error("chain call failed"));

      await messageClaimingProcessor.process();

      // Nonce must be rolled back when claim throws, regardless of DB state.
      expect(nonceManager.rollbackNonce).toHaveBeenCalledWith(101);
    });

    it("Should log 'underpriced will retry later' when message is already FEE_UNDERPRICED", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      const underpricedMsg = new Message({
        ...testUnderpricedAnchoredMessage,
        status: MessageStatus.FEE_UNDERPRICED,
      });
      getNextMessageToClaim.mockResolvedValue(underpricedMsg);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(transactionValidationService, "evaluateTransaction").mockResolvedValueOnce({
        hasZeroFee: false,
        isRateLimitExceeded: false,
        isUnderPriced: true,
        isForSponsorship: false,
        estimatedGasLimit: 100_000n,
        threshold: 10,
        maxPriorityFeePerGas: 1000000000n,
        maxFeePerGas: 1000000000n,
      });

      await messageClaimingProcessor.process();

      expect(loggerWarnSpy).toHaveBeenCalledWith("Message is underpriced, will retry later.", {
        messageHash: underpricedMsg.messageHash,
      });
    });

    it("Should set message as NON_EXECUTABLE when error is not retryable", async () => {
      const mockErrorParser = mock<IErrorParser>();
      mockErrorParser.parse.mockReturnValue({ retryable: false, message: "non-retryable" });
      const localRepo = mock<IMessageRepository>();
      const localGetNext = jest.fn();
      const localAnchoredMessage = new Message(testAnchoredMessage);

      const localProcessor = new MessageClaimingProcessor(
        lineaRollupContractMock,
        nonceManager,
        localRepo,
        localGetNext,
        transactionValidationService,
        mockErrorParser,
        {
          profitMargin: DEFAULT_PROFIT_MARGIN,
          maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
          retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
          maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
          direction: Direction.L2_TO_L1,
          originContractAddress: TEST_CONTRACT_ADDRESS_2,
        },
        logger,
      );

      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      localGetNext.mockResolvedValue(localAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockRejectedValue(new Error("non-retryable"));

      await localProcessor.process();

      expect(localRepo.updateMessage).toHaveBeenCalledWith(
        expect.objectContaining({ status: MessageStatus.NON_EXECUTABLE }),
      );
    });

    it("Should rollback nonce and log error if claim throws", async () => {
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const actionRejectedError = {
        code: "ACTION_REJECTED",
        shortMessage: "action rejected error for testing",
      };

      const messageRepositorySaveSpy = jest
        .spyOn(messageRepository, "updateMessage")
        .mockResolvedValueOnce()
        .mockRejectedValue(actionRejectedError);
      const rollbackSpy = jest.spyOn(nonceManager, "rollbackNonce");

      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      // claim() succeeds; the subsequent DB write is what fails
      jest.spyOn(lineaRollupContractMock, "claim").mockResolvedValue({
        hash: TEST_TRANSACTION_HASH,
        nonce: 101,
        gasLimit: 100_000n,
        maxFeePerGas: 1000000000n,
        maxPriorityFeePerGas: 1000000000n,
      });
      const expectedLoggingMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedLoggingMessage);
      expect(rollbackSpy).toHaveBeenCalledWith(101);
      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error processing message claim.", {
        error: actionRejectedError,
        parsedError: errorParser.parse(actionRejectedError),
        messageHash: expectedLoggingMessage.messageHash,
      });
    });
  });

  describe("edge cases", () => {
    it("Should handle null maxFeePerGas and maxPriorityFeePerGas from claim transaction", async () => {
      const freshMessage = new Message({
        messageSender: testAnchoredMessage.messageSender,
        destination: testAnchoredMessage.destination,
        fee: testAnchoredMessage.fee,
        value: testAnchoredMessage.value,
        messageNonce: testAnchoredMessage.messageNonce,
        calldata: testAnchoredMessage.calldata,
        messageHash: testAnchoredMessage.messageHash,
        contractAddress: testAnchoredMessage.contractAddress,
        sentBlockNumber: testAnchoredMessage.sentBlockNumber,
        direction: testAnchoredMessage.direction,
        status: MessageStatus.ANCHORED,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
      });
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(freshMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      jest.spyOn(lineaRollupContractMock, "claim").mockResolvedValue({
        hash: TEST_TRANSACTION_HASH,
        nonce: 101,
        gasLimit: 100_000n,
        maxFeePerGas: null as unknown as bigint,
        maxPriorityFeePerGas: null as unknown as bigint,
      });

      await messageClaimingProcessor.process();

      // null ?? undefined = undefined, and edit() skips undefined values,
      // so claimTxMaxFeePerGas remains undefined (fresh message has no prior value)
      const lastUpdateCall = messageRepositorySaveSpy.mock.calls[messageRepositorySaveSpy.mock.calls.length - 1][0];
      expect(lastUpdateCall.claimTxMaxFeePerGas).toBeUndefined();
      expect(lastUpdateCall.claimTxMaxPriorityFeePerGas).toBeUndefined();
      expect(lastUpdateCall.claimTxHash).toBe(TEST_TRANSACTION_HASH);
      expect(nonceManager.commitNonce).toHaveBeenCalledWith(101);
    });

    it("Should log error without messageHash when getNextMessageToClaim throws", async () => {
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const fetchError = new Error("failed to fetch next message");
      getNextMessageToClaim.mockRejectedValue(fetchError);

      await messageClaimingProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error processing message claim.", {
        error: fetchError,
        parsedError: errorParser.parse(fetchError),
      });
    });
  });

  describe("process with sponsorship", () => {
    beforeEach(() => {
      transactionValidationService = new EthereumTransactionValidationService(
        lineaRollupContractMock,
        gasProvider,
        {
          profitMargin: DEFAULT_PROFIT_MARGIN,
          maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
          isPostmanSponsorshipEnabled: true,
          maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
        },
        logger,
      );
      messageClaimingProcessor = new MessageClaimingProcessor(
        lineaRollupContractMock,
        nonceManager,
        messageRepository,
        getNextMessageToClaim,
        transactionValidationService,
        errorParser,
        {
          profitMargin: DEFAULT_PROFIT_MARGIN,
          maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
          retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
          maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
          direction: Direction.L2_TO_L1,
          originContractAddress: TEST_CONTRACT_ADDRESS_2,
        },
        logger,
      );
    });

    it("Should successfully claim message with fee", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      const expectedLoggingMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        updatedAt: mockedDate,
        isForSponsorship: false,
      });
      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedLoggingMessage);
    });

    it("Should successfully claim message with zero fee", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testZeroFeeAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      jest.spyOn(lineaRollupContractMock, "claim").mockResolvedValue({
        hash: TEST_TRANSACTION_HASH,
        nonce: 101,
        gasLimit: 100_000n,
        maxFeePerGas: 1000000000n,
        maxPriorityFeePerGas: 1000000000n,
      });
      const expectedFinalMessage = new Message({
        ...testZeroFeeAnchoredMessage,
        claimGasEstimationThreshold: 0,
        isForSponsorship: true,
        claimTxNonce: 101,
        status: MessageStatus.PENDING,
        claimTxCreationDate: mockedDate,
        claimTxGasLimit: 100000,
        claimTxMaxFeePerGas: 1000000000n,
        claimTxMaxPriorityFeePerGas: 1000000000n,
        claimTxHash: TEST_TRANSACTION_HASH,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedFinalMessage);
      expect(nonceManager.commitNonce).toHaveBeenCalledWith(101);
    });

    it("Should successfully claim message with underpriced fee", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testUnderpricedAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "claim").mockResolvedValue({
        hash: TEST_TRANSACTION_HASH,
        nonce: 101,
        gasLimit: 100_000n,
        maxFeePerGas: 1000000000n,
        maxPriorityFeePerGas: 1000000000n,
      });
      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          claimGasEstimationThreshold: 10,
          isForSponsorship: true,
          claimTxNonce: 101,
          status: MessageStatus.PENDING,
          claimTxGasLimit: 100000,
          claimTxMaxFeePerGas: 1000000000n,
          claimTxMaxPriorityFeePerGas: 1000000000n,
          claimTxHash: TEST_TRANSACTION_HASH,
        }),
      );
      expect(nonceManager.commitNonce).toHaveBeenCalledWith(101);
    });

    it("Should successfully claim message on a specified contract address if specified", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(messageRepository, "updateMessage");
      jest.spyOn(nonceManager, "acquireNonce").mockResolvedValue(101);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      getNextMessageToClaim.mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      const expectedLoggingMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        updatedAt: mockedDate,
        isForSponsorship: false,
      });

      const messageClaimingProcessorWithSpecifiedClaimAddress = new MessageClaimingProcessor(
        lineaRollupContractMock,
        nonceManager,
        messageRepository,
        getNextMessageToClaim,
        transactionValidationService,
        errorParser,
        {
          claimViaAddress: TEST_ADDRESS_2,
          profitMargin: DEFAULT_PROFIT_MARGIN,
          maxNumberOfRetries: DEFAULT_MAX_NUMBER_OF_RETRIES,
          retryDelayInSeconds: DEFAULT_RETRY_DELAY_IN_SECONDS,
          maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
          direction: Direction.L2_TO_L1,
          originContractAddress: TEST_CONTRACT_ADDRESS_2,
        },
        logger,
      );
      await messageClaimingProcessorWithSpecifiedClaimAddress.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedLoggingMessage);
    });
  });
});
