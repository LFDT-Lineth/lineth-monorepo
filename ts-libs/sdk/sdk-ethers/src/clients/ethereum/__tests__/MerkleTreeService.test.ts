import { describe, beforeEach } from "@jest/globals";
import { Wallet } from "ethers";
import { MockProxy, mock } from "jest-mock-extended";

import { LinethRollup, LinethRollup__factory } from "../../../contracts/typechain";
import {
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MERKLE_ROOT_2,
  TEST_MESSAGE_HASH,
  TEST_MESSAGE_HASH_2,
} from "../../../utils/testing/constants/common";
import { testL2MessagingBlockAnchoredEvent, testMessageSentEvent } from "../../../utils/testing/constants/events";
import {
  generateL2MerkleTreeAddedLog,
  generateL2MessagingBlockAnchoredLog,
  generateLinethRollupClient,
  generateTransactionReceiptWithLogs,
} from "../../../utils/testing/helpers";
import { EthersL2MessageServiceLogClient } from "../../linea/EthersL2MessageServiceLogClient";
import { LineaProvider, Provider } from "../../providers";
import { EthersLinethRollupLogClient } from "../EthersLinethRollupLogClient";
import { MerkleTreeService } from "../MerkleTreeService";

describe("MerkleTreeService", () => {
  let providerMock: MockProxy<Provider>;
  let l2ProviderMock: MockProxy<LineaProvider>;
  let walletMock: MockProxy<Wallet>;
  let linethRollupMock: MockProxy<LinethRollup>;

  let merkleTreeService: MerkleTreeService;
  let linethRollupLogClient: EthersLinethRollupLogClient;
  let l2MessageServiceLogClient: EthersL2MessageServiceLogClient;

  beforeEach(() => {
    providerMock = mock<Provider>();
    l2ProviderMock = mock<LineaProvider>();
    walletMock = mock<Wallet>();
    linethRollupMock = mock<LinethRollup>();

    const clients = generateLinethRollupClient(
      providerMock,
      l2ProviderMock,
      TEST_CONTRACT_ADDRESS_1,
      TEST_CONTRACT_ADDRESS_2,
      "read-write",
      walletMock,
    );
    merkleTreeService = clients.merkleTreeService;
    l2MessageServiceLogClient = clients.l2MessageServiceLogClient;
    linethRollupLogClient = clients.linethRollupLogClient;
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("getMessageSiblings", () => {
    it("should throw a BaseError when message hash not found in messages", () => {
      const messageHash = TEST_MESSAGE_HASH;
      const messageHashes = [TEST_MESSAGE_HASH_2];

      expect(() => merkleTreeService.getMessageSiblings(messageHash, messageHashes, 5)).toThrow(
        "Message hash not found in messages.",
      );
    });
  });

  describe("getMessageProof", () => {
    it("should throw a BaseError if merkle tree build failed", async () => {
      const messageHash = TEST_MESSAGE_HASH;
      const transactionReceipt = generateTransactionReceiptWithLogs(undefined, [
        generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT_2, 5),
        generateL2MessagingBlockAnchoredLog(10n),
      ]);
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByBlockRange")
        .mockResolvedValue([testMessageSentEvent]);
      jest
        .spyOn(linethRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);
      jest.spyOn(LinethRollup__factory, "connect").mockReturnValueOnce(linethRollupMock);

      await expect(merkleTreeService.getMessageProof(messageHash)).rejects.toThrow("Merkle tree build failed.");
    });
  });
});
