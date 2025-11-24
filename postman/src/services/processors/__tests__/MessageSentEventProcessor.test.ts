import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { TestLogger } from "../../../utils/testing/helpers";
import { Direction, MessageSent } from "@consensys/linea-sdk";
import { MessageStatus } from "../../../core/enums";
import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  testL1NetworkConfig,
  testMessageSentEvent,
  testMessageSentEventWithCallData,
} from "../../../utils/testing/constants";
import { IProvider } from "../../../core/clients/blockchain/IProvider";
import { MessageSentEventProcessorConfig } from "../../../core/services/processors/IMessageSentEventProcessor";
import { MessageSentEventProcessor } from "../MessageSentEventProcessor";
import { ILineaRollupLogClient } from "../../../core/clients/blockchain/ethereum/ILineaRollupLogClient";
import { MessageFactory } from "../../../core/entities/MessageFactory";
import {
  Block,
  ContractTransactionResponse,
  Interface,
  JsonRpcProvider,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { IMessageDBService } from "../../../core/persistence/IMessageDBService";
import { ILogger } from "@consensys/linea-shared-utils";
import { IL2MessageServiceLogClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceLogClient";

class TestMessageSentEventProcessor extends MessageSentEventProcessor {
  constructor(
    databaseService: IMessageDBService<ContractTransactionResponse>,
    logClient: ILineaRollupLogClient | IL2MessageServiceLogClient,
    provider: IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, JsonRpcProvider>,
    public readonly config: MessageSentEventProcessorConfig,
    logger: ILogger,
  ) {
    super(databaseService, logClient, provider, config, logger);
  }

  public shouldProcessMessage(
    message: MessageSent,
    messageHash: string,
    filters?: { criteriaExpression: string; calldataFunctionInterface: string },
  ): boolean {
    return super.shouldProcessMessage(message, messageHash, filters);
  }
}

describe("TestMessageSentEventProcessor", () => {
  let messageSentEventProcessor: TestMessageSentEventProcessor;
  const databaseService = mock<EthereumMessageDBService>();
  const l1LogClientMock = mock<ILineaRollupLogClient>();
  const provider =
    mock<IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, JsonRpcProvider>>();
  const logger = new TestLogger(MessageSentEventProcessor.name);

  beforeEach(() => {
    messageSentEventProcessor = new TestMessageSentEventProcessor(
      databaseService,
      l1LogClientMock,
      provider,
      {
        direction: Direction.L1_TO_L2,
        maxBlocksToFetchLogs: testL1NetworkConfig.listener.maxBlocksToFetchLogs,
        blockConfirmation: testL1NetworkConfig.listener.blockConfirmation,
        isEOAEnabled: testL1NetworkConfig.isEOAEnabled,
        isCalldataEnabled: testL1NetworkConfig.isCalldataEnabled,
      },
      logger,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("process", () => {
    it("Should insert message with status as sent into repository if the message is not excluded", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      const messageRepositoryInsertSpy = jest.spyOn(databaseService, "insertMessage");
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l1LogClientMock, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);
      const expectedMessageToInsert = MessageFactory.createMessage({
        ...testMessageSentEvent,
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.SENT,
        claimNumberOfRetry: 0,
      });

      await messageSentEventProcessor.process(200, 0);

      expect(loggerInfoSpy).toHaveBeenCalledTimes(3);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledWith(expectedMessageToInsert);
    });

    it("Should insert message with status as excluded into repository if the message is excluded", async () => {
      messageSentEventProcessor = new TestMessageSentEventProcessor(
        databaseService,
        l1LogClientMock,
        provider,
        {
          direction: Direction.L1_TO_L2,
          maxBlocksToFetchLogs: testL1NetworkConfig.listener.maxBlocksToFetchLogs,
          blockConfirmation: testL1NetworkConfig.listener.blockConfirmation,
          isEOAEnabled: !testL1NetworkConfig.isEOAEnabled,
          isCalldataEnabled: testL1NetworkConfig.isCalldataEnabled,
        },
        logger,
      );
      const loggerInfoSpy = jest.spyOn(logger, "info");
      const messageRepositoryInsertSpy = jest.spyOn(databaseService, "insertMessage");
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l1LogClientMock, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);
      const expectedMessageToInsert = MessageFactory.createMessage({
        ...testMessageSentEvent,
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.EXCLUDED,
        claimNumberOfRetry: 0,
      });

      await messageSentEventProcessor.process(0, 0);

      expect(loggerInfoSpy).toHaveBeenCalledTimes(3);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledWith(expectedMessageToInsert);
    });

    it("Should insert message with status as excluded into repository if the message is excluded becuase of events filters", async () => {
      messageSentEventProcessor = new TestMessageSentEventProcessor(
        databaseService,
        l1LogClientMock,
        provider,
        {
          direction: Direction.L1_TO_L2,
          maxBlocksToFetchLogs: testL1NetworkConfig.listener.maxBlocksToFetchLogs,
          blockConfirmation: testL1NetworkConfig.listener.blockConfirmation,
          isEOAEnabled: testL1NetworkConfig.isEOAEnabled,
          isCalldataEnabled: true,
          eventFilters: {
            fromAddressFilter: TEST_ADDRESS_1,
            toAddressFilter: TEST_ADDRESS_2,
            calldataFilter: {
              criteriaExpression: `calldata.funcSignature == "0x26dfbc20" and calldata.amount == 0`,
              calldataFunctionInterface: "function receiveFromOtherLayer(address recipient, uint256 amount)",
            },
          },
        },
        logger,
      );
      const loggerInfoSpy = jest.spyOn(logger, "info");
      const messageRepositoryInsertSpy = jest.spyOn(databaseService, "insertMessage");
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l1LogClientMock, "getMessageSentEvents").mockResolvedValue([
        testMessageSentEvent,
        {
          ...testMessageSentEvent,
          calldata:
            "0x26dfbc200000000000000000000000005eeea0e70ffe4f5419477056023c4b0aca01656200000000000000000000000000000000000000000000000000000000000186a0",
        },
      ]);
      const expectedMessage1ToInsert = MessageFactory.createMessage({
        ...testMessageSentEvent,
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.SENT,
        claimNumberOfRetry: 0,
      });

      const expectedMessage2ToInsert = MessageFactory.createMessage({
        ...{
          ...testMessageSentEvent,
          calldata:
            "0x26dfbc200000000000000000000000005eeea0e70ffe4f5419477056023c4b0aca01656200000000000000000000000000000000000000000000000000000000000186a0",
        },
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.EXCLUDED,
        claimNumberOfRetry: 0,
      });

      await messageSentEventProcessor.process(0, 0);

      expect(loggerInfoSpy).toHaveBeenCalledTimes(3);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledTimes(2);
      expect(messageRepositoryInsertSpy).toHaveBeenNthCalledWith(1, expectedMessage1ToInsert);
      expect(messageRepositoryInsertSpy).toHaveBeenNthCalledWith(2, expectedMessage2ToInsert);
    });

    it("Should insert message with calldata with status as sent into repository if calldata is enabled", async () => {
      messageSentEventProcessor = new TestMessageSentEventProcessor(
        databaseService,
        l1LogClientMock,
        provider,
        {
          direction: Direction.L1_TO_L2,
          maxBlocksToFetchLogs: testL1NetworkConfig.listener.maxBlocksToFetchLogs,
          blockConfirmation: testL1NetworkConfig.listener.blockConfirmation,
          isEOAEnabled: testL1NetworkConfig.isEOAEnabled,
          isCalldataEnabled: !testL1NetworkConfig.isCalldataEnabled,
        },
        logger,
      );
      const loggerInfoSpy = jest.spyOn(logger, "info");
      const messageRepositoryInsertSpy = jest.spyOn(databaseService, "insertMessage");
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l1LogClientMock, "getMessageSentEvents").mockResolvedValue([testMessageSentEventWithCallData]);
      const expectedMessageToInsert = MessageFactory.createMessage({
        ...testMessageSentEventWithCallData,
        sentBlockNumber: testMessageSentEventWithCallData.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.SENT,
        claimNumberOfRetry: 0,
      });

      await messageSentEventProcessor.process(0, 0);

      expect(loggerInfoSpy).toHaveBeenCalledTimes(3);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledWith(expectedMessageToInsert);
    });
  });

  describe("shouldProcessMessage", () => {
    const funcFragment = "function receiveFromOtherLayer(address recipient, uint256 amount)";

    const encodedCalldata = new Interface([funcFragment]).encodeFunctionData(funcFragment, [
      "0x5eeea0e70ffe4f5419477056023c4b0aca016562",
      100000n,
    ]);

    it("Should return true if calldata is empty and EOA is enabled", () => {
      const result = messageSentEventProcessor.shouldProcessMessage(
        testMessageSentEvent,
        testMessageSentEvent.messageHash,
      );
      expect(result).toBeTruthy();
    });

    it("Should return false if calldata is empty and EOA is disabled", () => {
      messageSentEventProcessor.config.isEOAEnabled = false;
      const result = messageSentEventProcessor.shouldProcessMessage(
        testMessageSentEvent,
        testMessageSentEvent.messageHash,
      );
      expect(result).toBeFalsy();
    });

    it("Should return true if calldata is not empty and calldata option is enabled", () => {
      messageSentEventProcessor.config.isCalldataEnabled = true;
      const result = messageSentEventProcessor.shouldProcessMessage(
        { ...testMessageSentEvent, calldata: "0x1111111111" },
        testMessageSentEvent.messageHash,
      );
      expect(result).toBeTruthy();
    });

    it("Should return false if calldata is not empty and calldata option is disabled", () => {
      const result = messageSentEventProcessor.shouldProcessMessage(
        { ...testMessageSentEvent, calldata: "0x1111111111" },
        testMessageSentEvent.messageHash,
      );
      expect(result).toBeFalsy();
    });

    it("Should return false if EOA and calldata options are disabled", () => {
      messageSentEventProcessor.config.isEOAEnabled = false;
      const result = messageSentEventProcessor.shouldProcessMessage(
        { ...testMessageSentEvent, calldata: "0x1111111111" },
        testMessageSentEvent.messageHash,
      );
      expect(result).toBeFalsy();
    });

    it("Should return false if event filter criteria is not correctly formatted", () => {
      messageSentEventProcessor.config.isCalldataEnabled = true;

      const result = messageSentEventProcessor.shouldProcessMessage(
        {
          ...testMessageSentEvent,
          calldata: encodedCalldata,
        },
        testMessageSentEvent.messageHash,
        {
          criteriaExpression: `calldata.funcSignature == 0x26dfbc20 and calldata.amount > 0`,
          calldataFunctionInterface: funcFragment,
        },
      );
      expect(result).toBeFalsy();
    });

    it("Should return false if event filter criteria is false", () => {
      messageSentEventProcessor.config.isCalldataEnabled = true;

      const result = messageSentEventProcessor.shouldProcessMessage(
        {
          ...testMessageSentEvent,
          calldata: encodedCalldata,
        },
        testMessageSentEvent.messageHash,
        {
          criteriaExpression: `calldata.funcSignature == "0x26dfbc20" and calldata.amount == 0`,
          calldataFunctionInterface: funcFragment,
        },
      );
      expect(result).toBeFalsy();
    });

    it("Should return true if event filter criteria is true", () => {
      messageSentEventProcessor.config.isCalldataEnabled = true;

      const result = messageSentEventProcessor.shouldProcessMessage(
        {
          ...testMessageSentEvent,
          calldata: encodedCalldata,
        },
        testMessageSentEvent.messageHash,
        {
          criteriaExpression: `calldata.funcSignature == "0x26dfbc20" and calldata.amount > 0`,
          calldataFunctionInterface: funcFragment,
        },
      );
      expect(result).toBeTruthy();
    });
  });
});
