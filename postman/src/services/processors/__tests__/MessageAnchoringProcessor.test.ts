import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
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
import { OnChainMessageStatus } from "@consensys/linea-sdk";
import { TestLogger } from "../../../utils/testing/helpers";
import { IMessageAnchoringProcessor } from "../../../core/services/processors/IMessageAnchoringProcessor";
import { MessageStatus } from "../../../core/enums";
import { testL1NetworkConfig, testL2NetworkConfig, testMessage } from "../../../utils/testing/constants";
import { MessageAnchoringProcessor } from "../MessageAnchoringProcessor";
import { IMessageServiceContract } from "../../../core/services/contracts/IMessageServiceContract";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { IProvider } from "../../../core/clients/blockchain/IProvider";

describe("TestMessageAnchoringProcessor", () => {
  let anchoringProcessor: IMessageAnchoringProcessor;
  const databaseService = mock<EthereumMessageDBService>();
  const l2ContractClientMock =
    mock<
      IMessageServiceContract<
        Overrides,
        TransactionReceipt,
        TransactionResponse,
        ContractTransactionResponse,
        ErrorDescription
      >
    >();
  const provider =
    mock<IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, JsonRpcProvider>>();
  const logger = new TestLogger(MessageAnchoringProcessor.name);

  beforeEach(() => {
    anchoringProcessor = new MessageAnchoringProcessor(
      l2ContractClientMock,
      provider,
      databaseService,
      {
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
      jest.spyOn(databaseService, "getNFirstMessagesSent").mockResolvedValue([]);

      await anchoringProcessor.process();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as warning if getNFirstMessageSent returns a list longer than maxFetchMessagesFromDb", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");
      const maxFetchMessagesFromDb = testL2NetworkConfig.listener.maxFetchMessagesFromDb;
      jest
        .spyOn(databaseService, "getNFirstMessagesSent")
        .mockResolvedValue(Array(maxFetchMessagesFromDb).map(() => testMessage));

      await anchoringProcessor.process();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith(
        "Limit of messages sent to listen reached (%s).",
        maxFetchMessagesFromDb,
      );
    });

    it("Should log as info and call saveMessages if returned messageStatus is CLAIMABLE", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      jest.spyOn(databaseService, "getNFirstMessagesSent").mockResolvedValue([testMessage]);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      jest.spyOn(l2ContractClientMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      const testMessageEditSpy = jest.spyOn(testMessage, "edit");
      const messageRepositoryMockSaveSpy = jest.spyOn(databaseService, "saveMessages");

      await anchoringProcessor.process();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Message has been anchored: messageHash=%s", testMessage.messageHash);
      expect(testMessageEditSpy).toHaveBeenCalledTimes(1);
      expect(testMessageEditSpy).toHaveBeenCalledWith({ status: MessageStatus.ANCHORED });
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledWith([testMessage]);
    });

    it("Should log as info and call saveMessages if returned messageStatus is CLAIMED", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      jest.spyOn(databaseService, "getNFirstMessagesSent").mockResolvedValue([testMessage]);
      jest.spyOn(provider, "getBlockNumber").mockResolvedValue(10);
      jest.spyOn(l2ContractClientMock, "getMessageStatus").mockResolvedValue(OnChainMessageStatus.CLAIMED);
      const testMessageEditSpy = jest.spyOn(testMessage, "edit");
      const messageRepositoryMockSaveSpy = jest.spyOn(databaseService, "saveMessages");

      await anchoringProcessor.process();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith(
        "Message has already been claimed: messageHash=%s",
        testMessage.messageHash,
      );
      expect(testMessageEditSpy).toHaveBeenCalledTimes(1);
      expect(testMessageEditSpy).toHaveBeenCalledWith({ status: MessageStatus.CLAIMED_SUCCESS });
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledTimes(1);
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledWith([testMessage]);
    });

    it("Should log as error if getCurrentBlockNumber throws error", async () => {
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const error = new Error("Error for testing");
      jest.spyOn(databaseService, "getNFirstMessagesSent").mockResolvedValue([testMessage]);
      jest.spyOn(provider, "getBlockNumber").mockRejectedValue(error);
      const messageRepositoryMockSaveSpy = jest.spyOn(databaseService, "saveMessages");

      await anchoringProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith(error);
      expect(messageRepositoryMockSaveSpy).toHaveBeenCalledTimes(0);
    });
  });
});
