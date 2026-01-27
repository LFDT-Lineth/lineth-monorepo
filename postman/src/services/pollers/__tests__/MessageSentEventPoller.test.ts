import { Provider, DefaultGasProvider, Direction, wait } from "@consensys/linea-sdk";
import { describe, it, beforeEach } from "@jest/globals";
import { Block, TransactionReceipt, TransactionRequest, TransactionResponse } from "ethers";
import { mock } from "jest-mock-extended";

import { IProvider } from "../../../core/clients/blockchain/IProvider";
import {
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_INITIAL_FROM_BLOCK,
  DEFAULT_LISTENER_INTERVAL,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
} from "../../../core/constants";
import { DatabaseErrorType, DatabaseRepoName } from "../../../core/enums";
import { DatabaseAccessError } from "../../../core/errors";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { IPoller } from "../../../core/services/pollers/IPoller";
import { IMessageSentEventProcessor } from "../../../core/services/processors/IMessageSentEventProcessor";
import { rejectedMessageProps, testL1NetworkConfig, testMessage } from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { MessageSentEventPoller } from "../MessageSentEventPoller";

describe("TestMessageSentEventPoller", () => {
  let testMessageSentEventPoller: IPoller;
  let databaseService: EthereumMessageDBService;

  const eventProcessorMock = mock<IMessageSentEventProcessor>();
  const provider = mock<IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, Provider>>();
  const logger = new TestLogger(MessageSentEventPoller.name);

  beforeEach(() => {
    const gasProvider = new DefaultGasProvider(provider, {
      maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
      gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
      enforceMaxGasFee: false,
    });
    databaseService = new EthereumMessageDBService(gasProvider, mock<IMessageRepository<unknown>>());
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
      expect(loggerWarnSpy).toHaveBeenCalledWith("%s has already started.", MessageSentEventPoller.name);

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
      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting %s %s...", Direction.L1_TO_L2, MessageSentEventPoller.name);
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
      expect(loggerErrorSpy).toHaveBeenCalledWith(error);
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
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Something went wrong with database access. Restarting fromBlockNum=%s and fromLogIndex=%s and errorMessage=%s",
        0,
        1,
        new DatabaseAccessError(
          DatabaseRepoName.MessageRepository,
          DatabaseErrorType.Read,
          new Error("read database error."),
        ).message,
      );
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
      const loggerWarnOrErrorSpy = jest.spyOn(logger, "warnOrError");

      await testMessageSentEventPoller.start();
      await wait(500);

      expect(loggerWarnOrErrorSpy).toHaveBeenCalled();
      expect(loggerWarnOrErrorSpy).toHaveBeenCalledWith(error);
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
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        1,
        "Starting %s %s...",
        Direction.L1_TO_L2,
        MessageSentEventPoller.name,
      );
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        2,
        "Stopping %s %s...",
        Direction.L1_TO_L2,
        MessageSentEventPoller.name,
      );
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        3,
        "%s %s stopped.",
        Direction.L1_TO_L2,
        MessageSentEventPoller.name,
      );
    });
  });
});
