import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { IBlockProvider } from "../../../core/clients/blockchain/IProvider";
import { DEFAULT_INITIAL_FROM_BLOCK, DEFAULT_LISTENER_INTERVAL } from "../../../core/constants";
import { Direction, DatabaseErrorType, DatabaseRepoName } from "../../../core/enums";
import { DatabaseAccessError } from "../../../core/errors";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { IPoller } from "../../../core/services/pollers/IPoller";
import { IMessageSentEventProcessor } from "../../../core/services/processors/IMessageSentEventProcessor";
import { wait } from "../../../core/utils/shared";
import { rejectedMessageProps, testL1NetworkConfig, testMessage } from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { MessageSentEventPoller } from "../MessageSentEventPoller";

describe("TestMessageSentEventPoller", () => {
  let testMessageSentEventPoller: IPoller;
  const databaseService = mock<IMessageRepository>();

  const eventProcessorMock = mock<IMessageSentEventProcessor>();
  const provider = mock<IBlockProvider>();
  const logger = new TestLogger(MessageSentEventPoller.name);

  beforeEach(() => {
    testMessageSentEventPoller = new MessageSentEventPoller(
      eventProcessorMock,
      provider,
      databaseService,
      {
        direction: Direction.L1_TO_L2,
        pollingInterval: DEFAULT_LISTENER_INTERVAL,
        initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
        originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
      },
      logger,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("start", () => {
    it("Should return and log as warning if it has been started", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      jest.spyOn(databaseService, "getLatestMessageSent").mockResolvedValue(null);
      jest.spyOn(eventProcessorMock, "process").mockResolvedValue({
        nextFromBlock: 20,
        nextFromBlockLogIndex: 0,
      });

      testMessageSentEventPoller.start();
      await wait(500);
      await testMessageSentEventPoller.start();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("Poller has already started.", { name: MessageSentEventPoller.name });

      testMessageSentEventPoller.stop();
    });

    it("Should call process and log as info if it started successfully", async () => {
      const l1QuerierMockSpy = jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      const messageRepositoryMockSpy = jest
        .spyOn(databaseService, "getLatestMessageSent")
        .mockResolvedValue(testMessage);
      const eventProcessorMockSpy = jest.spyOn(eventProcessorMock, "process").mockResolvedValue({
        nextFromBlock: 20,
        nextFromBlockLogIndex: 0,
      });
      const loggerInfoSpy = jest.spyOn(logger, "info");

      testMessageSentEventPoller.start();
      await wait(500);

      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting poller.", {
        direction: Direction.L1_TO_L2,
        name: MessageSentEventPoller.name,
      });
      expect(l1QuerierMockSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryMockSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryMockSpy).toHaveBeenCalledWith(
        Direction.L1_TO_L2,
        testL1NetworkConfig.messageServiceContractAddress,
      );
      expect(eventProcessorMockSpy).toHaveBeenCalled();
      expect(eventProcessorMockSpy).toHaveBeenCalledWith(10, 0);

      testMessageSentEventPoller.stop();
    });

    it("Should log as warning if getCurrentBlockNumber throws error", async () => {
      const error = new Error("Other error for testing");
      const l1QuerierMockSpy = jest.spyOn(provider, "getBlockNumber").mockRejectedValue(error);
      const loggerErrorSpy = jest.spyOn(logger, "error");

      await testMessageSentEventPoller.start();
      await wait(500);

      expect(loggerErrorSpy).toHaveBeenCalled();
      expect(loggerErrorSpy).toHaveBeenCalledWith("Failed to get initial block number.", {
        error,
        direction: Direction.L1_TO_L2,
      });
      expect(l1QuerierMockSpy).toHaveBeenCalled();

      testMessageSentEventPoller.stop();
    });

    it("Should log as warning if process throws DatabaseAccessError", async () => {
      const l1QuerierMockSpy = jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      const messageRepositoryMockSpy = jest
        .spyOn(databaseService, "getLatestMessageSent")
        .mockResolvedValue(testMessage);
      const eventProcessorMockSpy = jest
        .spyOn(eventProcessorMock, "process")
        .mockRejectedValue(
          new DatabaseAccessError(
            DatabaseRepoName.MessageRepository,
            DatabaseErrorType.Read,
            new Error("read database error."),
            { ...rejectedMessageProps, logIndex: 1 },
          ),
        );
      const loggerWarnSpy = jest.spyOn(logger, "warn");

      await testMessageSentEventPoller.start();
      await wait(500);

      expect(loggerWarnSpy).toHaveBeenCalled();
      expect(loggerWarnSpy).toHaveBeenCalledWith("Something went wrong with database access. Restarting.", {
        fromBlockNum: 0,
        fromLogIndex: 1,
        errorMessage: new DatabaseAccessError(
          DatabaseRepoName.MessageRepository,
          DatabaseErrorType.Read,
          new Error("read database error."),
        ).message,
      });
      expect(l1QuerierMockSpy).toHaveBeenCalled();
      expect(messageRepositoryMockSpy).toHaveBeenCalled();
      expect(messageRepositoryMockSpy).toHaveBeenCalledWith(
        Direction.L1_TO_L2,
        testL1NetworkConfig.messageServiceContractAddress,
      );
      expect(eventProcessorMockSpy).toHaveBeenCalled();
      expect(eventProcessorMockSpy).toHaveBeenCalledWith(10, 0);

      testMessageSentEventPoller.stop();
    });

    it("Should log as warning or error if process throws Error", async () => {
      const l1QuerierMockSpy = jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      const messageRepositoryMockSpy = jest
        .spyOn(databaseService, "getLatestMessageSent")
        .mockResolvedValue(testMessage);
      const error = new Error("Other error for testing");
      const eventProcessorMockSpy = jest.spyOn(eventProcessorMock, "process").mockRejectedValue(error);
      const loggerErrorSpy = jest.spyOn(logger, "error");

      await testMessageSentEventPoller.start();
      await wait(500);

      expect(loggerErrorSpy).toHaveBeenCalled();
      expect(loggerErrorSpy).toHaveBeenCalledWith("Unexpected error processing events.", {
        error,
        direction: Direction.L1_TO_L2,
      });
      expect(l1QuerierMockSpy).toHaveBeenCalled();
      expect(messageRepositoryMockSpy).toHaveBeenCalled();
      expect(messageRepositoryMockSpy).toHaveBeenCalledWith(
        Direction.L1_TO_L2,
        testL1NetworkConfig.messageServiceContractAddress,
      );
      expect(eventProcessorMockSpy).toHaveBeenCalled();
      expect(eventProcessorMockSpy).toHaveBeenCalledWith(10, 0);

      testMessageSentEventPoller.stop();
    });
  });

  describe("initialFromBlock override", () => {
    it("Should use initialFromBlock when it exceeds DEFAULT_INITIAL_FROM_BLOCK", async () => {
      const overrideBlock = 100;
      const pollerWithOverride = new MessageSentEventPoller(
        eventProcessorMock,
        provider,
        databaseService,
        {
          direction: Direction.L1_TO_L2,
          pollingInterval: DEFAULT_LISTENER_INTERVAL,
          initialFromBlock: overrideBlock,
          originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
        },
        logger,
      );

      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      jest.spyOn(databaseService, "getLatestMessageSent").mockResolvedValue(testMessage);
      const eventProcessorMockSpy = jest.spyOn(eventProcessorMock, "process").mockResolvedValue({
        nextFromBlock: overrideBlock + 10,
        nextFromBlockLogIndex: 0,
      });

      pollerWithOverride.start();
      await wait(500);

      expect(eventProcessorMockSpy).toHaveBeenCalledWith(overrideBlock, 0);

      pollerWithOverride.stop();
    });
  });

  describe("recursive call coverage", () => {
    it("Should retry startProcessingEvents after initial error", async () => {
      const fastPoller = new MessageSentEventPoller(
        eventProcessorMock,
        provider,
        databaseService,
        {
          direction: Direction.L1_TO_L2,
          pollingInterval: 10,
          initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
          originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
        },
        logger,
      );

      let callCount = 0;
      jest.spyOn(provider, "getBlockNumber").mockImplementation(async () => {
        callCount++;
        if (callCount === 1) throw new Error("initial error");
        return 10;
      });
      jest.spyOn(databaseService, "getLatestMessageSent").mockResolvedValue(null);
      jest.spyOn(eventProcessorMock, "process").mockResolvedValue({ nextFromBlock: 20, nextFromBlockLogIndex: 0 });

      fastPoller.start();
      await wait(200);

      expect(callCount).toBeGreaterThanOrEqual(2);
      fastPoller.stop();
    });

    it("Should recursively call processEvents in the finally block", async () => {
      const fastPoller = new MessageSentEventPoller(
        eventProcessorMock,
        provider,
        databaseService,
        {
          direction: Direction.L1_TO_L2,
          pollingInterval: 10,
          initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
          originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
        },
        logger,
      );

      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      jest.spyOn(databaseService, "getLatestMessageSent").mockResolvedValue(null);
      const processSpy = jest.spyOn(eventProcessorMock, "process").mockResolvedValue({
        nextFromBlock: 20,
        nextFromBlockLogIndex: 0,
      });

      fastPoller.start();
      await wait(200);

      expect(processSpy.mock.calls.length).toBeGreaterThanOrEqual(2);
      fastPoller.stop();
    });
  });

  describe("stop", () => {
    it("Should return and log as info if it stopped successfully", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      testMessageSentEventPoller = new MessageSentEventPoller(
        eventProcessorMock,
        provider,
        databaseService,
        {
          direction: Direction.L1_TO_L2,
          pollingInterval: DEFAULT_LISTENER_INTERVAL,
          initialFromBlock: DEFAULT_INITIAL_FROM_BLOCK,
          originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
        },
        logger,
      );

      testMessageSentEventPoller.start();
      testMessageSentEventPoller.stop();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(3);
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(1, "Starting poller.", {
        direction: Direction.L1_TO_L2,
        name: MessageSentEventPoller.name,
      });
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(2, "Stopping poller.", {
        direction: Direction.L1_TO_L2,
        name: MessageSentEventPoller.name,
      });
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(3, "Poller stopped.", {
        direction: Direction.L1_TO_L2,
        name: MessageSentEventPoller.name,
      });
    });
  });
});
