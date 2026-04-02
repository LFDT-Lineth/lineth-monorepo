import { ILogger } from "@consensys/linea-shared-utils";
import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { IMessageSentEventLogClient } from "../../../core/clients/blockchain/ILogClient";
import { IBlockProvider } from "../../../core/clients/blockchain/IProvider";
import { Message } from "../../../core/entities/Message";
import { Direction, MessageStatus } from "../../../core/enums";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { ICalldataDecoder } from "../../../core/services/ICalldataDecoder";
import { MessageSentEventProcessorConfig } from "../../../core/services/processors/IMessageSentEventProcessor";
import { MessageSent } from "../../../core/types";
import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  testL1NetworkConfig,
  testMessageSentEvent,
  testMessageSentEventWithCallData,
} from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { MessageSentEventProcessor } from "../MessageSentEventProcessor";

class TestMessageSentEventProcessor extends MessageSentEventProcessor {
  constructor(
    databaseService: IMessageRepository,
    logClient: IMessageSentEventLogClient,
    provider: IBlockProvider,
    calldataDecoder: ICalldataDecoder,
    public readonly config: MessageSentEventProcessorConfig,
    logger: ILogger,
  ) {
    super(databaseService, logClient, provider, calldataDecoder, config, logger);
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
  const databaseService = mock<IMessageRepository>();
  const l1LogClientMock = mock<IMessageSentEventLogClient>();
  const provider = mock<IBlockProvider>();
  const calldataDecoder = mock<ICalldataDecoder>();
  const logger = new TestLogger(MessageSentEventProcessor.name);

  // Encoded calldata for: receiveFromOtherLayer(address, uint256)
  // selector: 0x26dfbc20, recipient: 0x5eeea0e70ffe4f5419477056023c4b0aca016562, amount: 100000
  const funcFragment = "function receiveFromOtherLayer(address recipient, uint256 amount)";
  const encodedCalldata =
    "0x26dfbc200000000000000000000000005eeea0e70ffe4f5419477056023c4b0aca01656200000000000000000000000000000000000000000000000000000000000186a0";

  beforeEach(() => {
    messageSentEventProcessor = new TestMessageSentEventProcessor(
      databaseService,
      l1LogClientMock,
      provider,
      calldataDecoder,
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
      const expectedMessageToInsert = new Message({
        ...testMessageSentEvent,
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.SENT,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
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
        calldataDecoder,
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
      const expectedMessageToInsert = new Message({
        ...testMessageSentEvent,
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.EXCLUDED,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
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
        calldataDecoder,
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
      // decoder returns amount=100000 which does NOT satisfy `amount == 0`
      calldataDecoder.decode.mockReturnValue({
        recipient: "0x5eeea0e70ffe4f5419477056023c4b0aca016562",
        amount: 100000n,
      });

      const loggerInfoSpy = jest.spyOn(logger, "info");
      const messageRepositoryInsertSpy = jest.spyOn(databaseService, "insertMessage");
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(100);
      jest.spyOn(l1LogClientMock, "getMessageSentEvents").mockResolvedValue([
        testMessageSentEvent,
        {
          ...testMessageSentEvent,
          calldata: encodedCalldata,
        },
      ]);
      const expectedMessage1ToInsert = new Message({
        ...testMessageSentEvent,
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.SENT,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
      });

      const expectedMessage2ToInsert = new Message({
        ...{
          ...testMessageSentEvent,
          calldata: encodedCalldata,
        },
        sentBlockNumber: testMessageSentEvent.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.EXCLUDED,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
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
        calldataDecoder,
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
      const expectedMessageToInsert = new Message({
        ...testMessageSentEventWithCallData,
        sentBlockNumber: testMessageSentEventWithCallData.blockNumber,
        direction: Direction.L1_TO_L2,
        status: MessageStatus.SENT,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
      });

      await messageSentEventProcessor.process(0, 0);

      expect(loggerInfoSpy).toHaveBeenCalledTimes(3);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryInsertSpy).toHaveBeenCalledWith(expectedMessageToInsert);
    });
  });

  describe("shouldProcessMessage", () => {
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
      calldataDecoder.decode.mockReturnValue({
        recipient: "0x5eeea0e70ffe4f5419477056023c4b0aca016562",
        amount: 100000n,
      });

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
      calldataDecoder.decode.mockReturnValue({
        recipient: "0x5eeea0e70ffe4f5419477056023c4b0aca016562",
        amount: 100000n,
      });

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

    it("Should handle array values in decoded calldata through convertBigInts", () => {
      messageSentEventProcessor.config.isCalldataEnabled = true;
      calldataDecoder.decode.mockReturnValue({
        recipients: ["0x5eeea0e70ffe4f5419477056023c4b0aca016562", "0x1234567890abcdef1234567890abcdef12345678"],
        amounts: [100000n, 200000n],
      });

      const result = messageSentEventProcessor.shouldProcessMessage(
        {
          ...testMessageSentEvent,
          calldata: encodedCalldata,
        },
        testMessageSentEvent.messageHash,
        {
          criteriaExpression: `calldata.funcSignature == "0x26dfbc20"`,
          calldataFunctionInterface: funcFragment,
        },
      );
      expect(result).toBeTruthy();
    });

    it("Should skip inherited properties in decoded calldata during convertBigInts", () => {
      messageSentEventProcessor.config.isCalldataEnabled = true;
      const decoded = Object.create({ inherited: 999n });
      decoded.amount = 100000n;
      calldataDecoder.decode.mockReturnValue(decoded);

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

    it("Should return true if event filter criteria is true", () => {
      messageSentEventProcessor.config.isCalldataEnabled = true;
      calldataDecoder.decode.mockReturnValue({
        recipient: "0x5eeea0e70ffe4f5419477056023c4b0aca016562",
        amount: 100000n,
      });

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
