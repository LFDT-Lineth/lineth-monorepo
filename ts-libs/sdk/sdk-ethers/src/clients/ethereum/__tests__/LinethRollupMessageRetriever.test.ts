import { describe, beforeEach } from "@jest/globals";
import { Wallet } from "ethers";
import { MockProxy, mock } from "jest-mock-extended";

import {
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "../../../utils/testing/constants/common";
import { testMessageSentEvent } from "../../../utils/testing/constants/events";
import { generateLinethRollupClient, generateTransactionReceipt } from "../../../utils/testing/helpers";
import { LineaProvider, Provider } from "../../providers";
import { EthersLinethRollupLogClient } from "../EthersLinethRollupLogClient";
import { LinethRollupMessageRetriever } from "../LinethRollupMessageRetriever";

describe("LinethRollupMessageRetriever", () => {
  let providerMock: MockProxy<Provider>;
  let l2ProviderMock: MockProxy<LineaProvider>;
  let walletMock: MockProxy<Wallet>;

  let messageRetriever: LinethRollupMessageRetriever;
  let linethRollupLogClient: EthersLinethRollupLogClient;

  beforeEach(() => {
    providerMock = mock<Provider>();
    l2ProviderMock = mock<LineaProvider>();
    walletMock = mock<Wallet>();

    const clients = generateLinethRollupClient(
      providerMock,
      l2ProviderMock,
      TEST_CONTRACT_ADDRESS_1,
      TEST_CONTRACT_ADDRESS_2,
      "read-write",
      walletMock,
    );
    messageRetriever = clients.messageRetriever;
    linethRollupLogClient = clients.linethRollupLogClient;
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("getMessageByMessageHash", () => {
    it("should return a MessageSent", async () => {
      jest.spyOn(linethRollupLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);

      const messageSentEvent = await messageRetriever.getMessageByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentEvent).toStrictEqual(testMessageSentEvent);
    });

    it("should return null if empty events returned", async () => {
      jest.spyOn(linethRollupLogClient, "getMessageSentEvents").mockResolvedValue([]);

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
      jest.spyOn(linethRollupLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);

      const messageSentEvents = await messageRetriever.getMessagesByTransactionHash(TEST_MESSAGE_HASH);

      expect(messageSentEvents).toStrictEqual([testMessageSentEvent]);
    });
  });

  describe("getTransactionReceiptByMessageHash", () => {
    it("should return null when message hash does not exist", async () => {
      jest.spyOn(linethRollupLogClient, "getMessageSentEvents").mockResolvedValue([]);

      const messageSentTxReceipt = await messageRetriever.getTransactionReceiptByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentTxReceipt).toStrictEqual(null);
    });

    it("should return null when transaction receipt does not exist", async () => {
      jest.spyOn(linethRollupLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(null);

      const messageSentTxReceipt = await messageRetriever.getTransactionReceiptByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentTxReceipt).toStrictEqual(null);
    });

    it("should return an array of messages when transaction hash exists and contains MessageSent events", async () => {
      const transactionReceipt = generateTransactionReceipt();
      jest.spyOn(linethRollupLogClient, "getMessageSentEvents").mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);

      const messageSentTxReceipt = await messageRetriever.getTransactionReceiptByMessageHash(TEST_MESSAGE_HASH);

      expect(messageSentTxReceipt).toStrictEqual(transactionReceipt);
    });
  });
});
