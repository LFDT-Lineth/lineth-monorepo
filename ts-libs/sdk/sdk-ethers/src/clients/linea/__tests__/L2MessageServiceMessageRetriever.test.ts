import { describe, beforeEach } from "@jest/globals";
import { Wallet } from "ethers";
import { MockProxy, mock } from "jest-mock-extended";

import {
  TEST_CONTRACT_ADDRESS_1,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "../../../utils/testing/constants/common";
import { testMessageSentEvent } from "../../../utils/testing/constants/events";
import { generateL2MessageServiceClient, generateTransactionReceipt } from "../../../utils/testing/helpers";
import { LineaProvider } from "../../providers";
import { EthersL2MessageServiceLogClient } from "../EthersL2MessageServiceLogClient";
import { L2MessageServiceMessageRetriever } from "../L2MessageServiceMessageRetriever";

describe("L2MessageServiceMessageRetriever", () => {
  let providerMock: MockProxy<LineaProvider>;
  let walletMock: MockProxy<Wallet>;

  let messageRetriever: L2MessageServiceMessageRetriever;
  let l2MessageServiceLogClient: EthersL2MessageServiceLogClient;

  beforeEach(() => {
    providerMock = mock<LineaProvider>();
    walletMock = mock<Wallet>();

    const clients = generateL2MessageServiceClient(providerMock, TEST_CONTRACT_ADDRESS_1, "read-write", walletMock);
    messageRetriever = clients.messageRetriever;
    l2MessageServiceLogClient = clients.l2MessageServiceLogClient;
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("getMessageByMessageHash", () => {
    it("should return a MessageSent", async () => {
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);

      const messageSentEvent = await messageRetriever.getMessageByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentEvent).toStrictEqual(testMessageSentEvent);
    });

    it("should return null if empty events returned", async () => {
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEvents").mockResolvedValue([]);

      const messageSentEvent = await messageRetriever.getMessageByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentEvent).toStrictEqual(null);
    });
  });

  describe("getMessagesByTransactionHash", () => {
    it("should return null when message hash does not exist", async () => {
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(null);

      const messageSentEvents = await messageRetriever.getMessagesByTransactionHash(TEST_TRANSACTION_HASH);

      expect(messageSentEvents).toStrictEqual(null);
    });

    it("should return an array of messages when transaction hash exists and contains MessageSent events", async () => {
      const transactionReceipt = generateTransactionReceipt();
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);

      const messageSentEvents = await messageRetriever.getMessagesByTransactionHash(TEST_MESSAGE_HASH);

      expect(messageSentEvents).toStrictEqual([testMessageSentEvent]);
    });
  });

  describe("getTransactionReceiptByMessageHash", () => {
    it("should return null when message hash does not exist", async () => {
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEvents").mockResolvedValue([]);

      const messageSentTxReceipt = await messageRetriever.getTransactionReceiptByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentTxReceipt).toStrictEqual(null);
    });

    it("should return null when transaction receipt does not exist", async () => {
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(null);

      const messageSentTxReceipt = await messageRetriever.getTransactionReceiptByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentTxReceipt).toStrictEqual(null);
    });

    it("should return an array of messages when transaction hash exists and contains MessageSent events", async () => {
      const transactionReceipt = generateTransactionReceipt();
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);

      const messageSentTxReceipt = await messageRetriever.getTransactionReceiptByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentTxReceipt).toStrictEqual(transactionReceipt);
    });
  });
});
