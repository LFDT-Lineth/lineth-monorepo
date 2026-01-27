import { DefaultGasProvider, Provider, Direction, OnChainMessageStatus } from "@consensys/linea-sdk";
import { describe, it, beforeEach } from "@jest/globals";
import {
  Block,
  ContractTransactionResponse,
  ErrorDescription,
  EthersError,
  Overrides,
  Signer,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { mock } from "jest-mock-extended";

import { ILineaRollupClient } from "../../../core/clients/blockchain/ethereum/ILineaRollupClient";
import { IProvider } from "../../../core/clients/blockchain/IProvider";
import {
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  DEFAULT_MAX_NUMBER_OF_RETRIES,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_PROFIT_MARGIN,
  DEFAULT_RETRY_DELAY_IN_SECONDS,
} from "../../../core/constants";
import { Message } from "../../../core/entities/Message";
import { MessageStatus } from "../../../core/enums";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { IMessageClaimingProcessor } from "../../../core/services/processors/IMessageClaimingProcessor";
import { ErrorParser } from "../../../utils/ErrorParser";
import {
  DEFAULT_MAX_FEE_PER_GAS,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_2,
  testAnchoredMessage,
  testClaimedMessage,
  testL2NetworkConfig,
  testUnderpricedAnchoredMessage,
  testZeroFeeAnchoredMessage,
} from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { EthereumTransactionValidationService } from "../../EthereumTransactionValidationService";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { MessageClaimingProcessor } from "../MessageClaimingProcessor";

describe("TestMessageClaimingProcessor", () => {
  let messageClaimingProcessor: IMessageClaimingProcessor;
  let gasProvider: DefaultGasProvider;
  let databaseService: EthereumMessageDBService;
  let transactionValidationService: EthereumTransactionValidationService;
  let mockedDate: Date;
  const lineaRollupContractMock =
    mock<
      ILineaRollupClient<
        Overrides,
        TransactionReceipt,
        TransactionResponse,
        ContractTransactionResponse,
        ErrorDescription
      >
    >();
  const provider = mock<IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, Provider>>();
  const signer = mock<Signer>();

  const logger = new TestLogger(MessageClaimingProcessor.name);

  beforeEach(() => {
    gasProvider = new DefaultGasProvider(provider, {
      maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
      gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
      enforceMaxGasFee: false,
    });
    databaseService = new EthereumMessageDBService(gasProvider, mock<IMessageRepository<unknown>>());
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
      signer,
      databaseService,
      transactionValidationService,
      {
        maxNonceDiff: 5,
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
    it("Should return and log as error if claim tx nonce is higher than the max diff", async () => {
      const loggerErrorSpy = jest.spyOn(logger, "error");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(80);

      await messageClaimingProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith(
        "Nonce returned from getNonce is an invalid value (e.g. null or undefined)",
      );
    });

    it("Should return without calling any get message status if getFirstMessageToClaim return null", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(null);

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as warning and save message as zero fee if message has zero fee", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testZeroFeeAnchoredMessage);
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
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Found message with zero fee. This message will not be processed: messageHash=%s",
        expectedLoggingMessage.messageHash,
      );
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as info and save message as claimed if message was claimed", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testClaimedMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      const expectedLoggingMessage = new Message(testClaimedMessage);
      const expectedSavedMessage = new Message({ ...testClaimedMessage, updatedAt: mockedDate });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledTimes(2);
      expect(loggerInfoSpy).toHaveBeenCalledWith(
        "Found already claimed message: messageHash=%s",
        expectedLoggingMessage.messageHash,
      );
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as warning and save message as non-executable if message gas limit was above max gas limit", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testAnchoredMessage);
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
        "Estimated gas limit is higher than the max allowed gas limit for this message: messageHash=%s messageInfo=%s estimatedGasLimit=%s maxAllowedGasLimit=%s",
        expectedLoggingMessage.messageHash,
        expectedLoggingMessage.toString(),
        undefined, // DEFAULT_MAX_CLAIM_GAS_LIMIT * 2n,
        testL2NetworkConfig.claiming.maxClaimGasLimit!.toString(),
      );
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedSavedMessage);
    });

    it("Should log as warning and save message as fee underpriced if message fee was underpriced", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testUnderpricedAnchoredMessage);
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
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Fee underpriced found in this message: messageHash=%s messageInfo=%s transactionGasLimit=%s maxFeePerGas=%s",
        expectedLoggingMessage.messageHash,
        expectedLoggingMessage.toString(),
        "100000",
        "1000000000",
      );
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(2);
      //expect(messageRepositorySaveSpy).toHaveBeenNthCalledWith(1, [expectedLoggingMessage]);
      expect(messageRepositorySaveSpy).toHaveBeenNthCalledWith(2, expectedSavedMessage);
    });

    it("Should log as warning and save message with reset claimGasEstimationThreshold if rate limit exceeded on L1", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testAnchoredMessage);
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
        "Rate limit exceeded for this message. It will be reprocessed later: messageHash=%s",
        expectedLoggingMessage.messageHash,
      );
    });

    it("Should update message if successful", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const messageRepositoryUpdateAtomicSpy = jest.spyOn(databaseService, "updateMessageWithClaimTxAtomic");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      const expectedLoggingMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedLoggingMessage);
      expect(messageRepositoryUpdateAtomicSpy).toHaveBeenCalledTimes(1);
    });

    it("Should log as warn or error if update message throws a ACTION_REJECTED error", async () => {
      const loggerWarnOrErrorSpy = jest.spyOn(logger, "warnOrError");
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const actionRejectedError = {
        code: "ACTION_REJECTED",
        shortMessage: "action rejected error for testing",
      };
      const messageRepositoryUpdateAtomicSpy = jest
        .spyOn(databaseService, "updateMessageWithClaimTxAtomic")
        .mockRejectedValue(actionRejectedError);
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      const expectedLoggingMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        updatedAt: mockedDate,
      });
      const expectedSavedMessage = new Message({
        ...testAnchoredMessage,
        claimGasEstimationThreshold: 10000000000,
        status: MessageStatus.NON_EXECUTABLE,
        updatedAt: mockedDate,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositorySaveSpy).toHaveBeenNthCalledWith(1, expectedLoggingMessage);
      expect(messageRepositorySaveSpy).toHaveBeenNthCalledWith(2, expectedSavedMessage);
      expect(messageRepositoryUpdateAtomicSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnOrErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnOrErrorSpy).toHaveBeenCalledWith(actionRejectedError, {
        parsedError: ErrorParser.parseErrorWithMitigation(actionRejectedError as EthersError),
        messageHash: expectedLoggingMessage.messageHash,
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
        signer,
        databaseService,
        transactionValidationService,
        {
          maxNonceDiff: 5,
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
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const messageRepositoryUpdateAtomicSpy = jest.spyOn(databaseService, "updateMessageWithClaimTxAtomic");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testAnchoredMessage);
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
      expect(messageRepositoryUpdateAtomicSpy).toHaveBeenCalledTimes(1);
    });

    it("Should successfully claim message with zero fee", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const messageRepositoryUpdateAtomicSpy = jest.spyOn(databaseService, "updateMessageWithClaimTxAtomic");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testZeroFeeAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      jest.spyOn(lineaRollupContractMock, "isRateLimitExceeded").mockResolvedValue(false);
      const expectedLoggingMessage = new Message({
        ...testZeroFeeAnchoredMessage,
        updatedAt: mockedDate,
        isForSponsorship: true,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedLoggingMessage);
      expect(messageRepositoryUpdateAtomicSpy).toHaveBeenCalledTimes(1);
    });

    it("Should successfully claim message with underpriced fee", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const messageRepositoryUpdateAtomicSpy = jest.spyOn(databaseService, "updateMessageWithClaimTxAtomic");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testUnderpricedAnchoredMessage);
      jest.spyOn(lineaRollupContractMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      jest.spyOn(lineaRollupContractMock, "estimateClaimGas").mockResolvedValue(100_000n);
      const expectedLoggingMessage = new Message({
        ...testUnderpricedAnchoredMessage,
        claimGasEstimationThreshold: 10,
        updatedAt: mockedDate,
        isForSponsorship: true,
      });

      await messageClaimingProcessor.process();

      expect(lineaRollupContractMsgStatusSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositorySaveSpy).toHaveBeenCalledWith(expectedLoggingMessage);
      expect(messageRepositoryUpdateAtomicSpy).toHaveBeenCalledTimes(1);
    });

    it("Should successfully claim message on a specified contract address if specified", async () => {
      const lineaRollupContractMsgStatusSpy = jest.spyOn(lineaRollupContractMock, "getMessageStatus");
      const messageRepositorySaveSpy = jest.spyOn(databaseService, "updateMessage");
      const messageRepositoryUpdateAtomicSpy = jest.spyOn(databaseService, "updateMessageWithClaimTxAtomic");
      jest.spyOn(databaseService, "getLastClaimTxNonce").mockResolvedValue(100);
      jest.spyOn(signer, "getNonce").mockResolvedValue(99);
      jest
        .spyOn(gasProvider, "getGasFees")
        .mockResolvedValue({ maxFeePerGas: 1000000000n, maxPriorityFeePerGas: 1000000000n });
      jest.spyOn(databaseService, "getMessageToClaim").mockResolvedValue(testAnchoredMessage);
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
        signer,
        databaseService,
        transactionValidationService,
        {
          claimViaAddress: TEST_ADDRESS_2,
          maxNonceDiff: 5,
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
      expect(messageRepositoryUpdateAtomicSpy).toHaveBeenCalledTimes(1);
    });
  });
});
