import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { Direction, OnChainMessageStatus, MessageStatus } from "../../../core/enums";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { IMessageStatusReader } from "../../../core/services/contracts/IMessageServiceContract";
import { IMessageAnchoringProcessor } from "../../../core/services/processors/IMessageAnchoringProcessor";
import { testL1NetworkConfig, testL2NetworkConfig, testMessage } from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { MessageAnchoringProcessor } from "../MessageAnchoringProcessor";

describe("TestMessageAnchoringProcessor", () => {
  let anchoringProcessor: IMessageAnchoringProcessor;
  const messageRepository = mock<IMessageRepository>();
  const l2ContractClientMock = mock<IMessageStatusReader>();
  const logger = new TestLogger(MessageAnchoringProcessor.name);
  beforeEach(() => {
    anchoringProcessor = new MessageAnchoringProcessor(
      l2ContractClientMock,
      messageRepository,
      {
        direction: Direction.L1_TO_L2,
        maxFetchMessagesFromDb: testL2NetworkConfig.listener.maxFetchMessagesFromDb,
        originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
      },
      logger,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("process", () => {
    it("Should return if getNFirstMessageSent returns empty list", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      jest.spyOn(messageRepository, "getNFirstMessagesByStatus").mockResolvedValue([]);

      await anchoringProcessor.process();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as warning if getNFirstMessageSent returns a list longer than maxFetchMessagesFromDb", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const maxFetchMessagesFromDb = testL2NetworkConfig.listener.maxFetchMessagesFromDb;
      jest
        .spyOn(messageRepository, "getNFirstMessagesByStatus")
        .mockResolvedValue(Array(maxFetchMessagesFromDb).map(() => testMessage));

      await anchoringProcessor.process();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("Limit of messages sent to listen reached.", {
        limit: maxFetchMessagesFromDb,
      });
    });

    it("Should log as info and call saveMessages if returned messageStatus is CLAIMABLE", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      jest.spyOn(messageRepository, "getNFirstMessagesByStatus").mockResolvedValue([testMessage]);
      jest.spyOn(l2ContractClientMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      const testMessageEditSpy = jest.spyOn(testMessage, "edit");
      const messageRepositoryMockSaveSpy = jest.spyOn(messageRepository, "saveMessages");

      await anchoringProcessor.process();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Message has been anchored.", {
        messageHash: testMessage.messageHash,
      });
      expect(testMessageEditSpy).toHaveBeenCalledTimes(1);
      expect(testMessageEditSpy).toHaveBeenCalledWith({ status: MessageStatus.ANCHORED });
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledWith([testMessage]);
    });

    it("Should log as info and call saveMessages if returned messageStatus is CLAIMED", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      jest.spyOn(messageRepository, "getNFirstMessagesByStatus").mockResolvedValue([testMessage]);
      jest.spyOn(l2ContractClientMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      const testMessageEditSpy = jest.spyOn(testMessage, "edit");
      const messageRepositoryMockSaveSpy = jest.spyOn(messageRepository, "saveMessages");

      await anchoringProcessor.process();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Message has already been claimed.", {
        messageHash: testMessage.messageHash,
      });
      expect(testMessageEditSpy).toHaveBeenCalledTimes(1);
      expect(testMessageEditSpy).toHaveBeenCalledWith({ status: MessageStatus.CLAIMED_SUCCESS });
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledWith([testMessage]);
    });
  });
});
