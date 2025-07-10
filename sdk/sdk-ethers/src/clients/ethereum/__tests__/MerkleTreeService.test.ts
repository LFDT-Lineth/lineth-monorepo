import { describe, beforeEach } from "@jest/globals";
import { Wallet } from "ethers";
import { MockProxy, mock } from "jest-mock-extended";
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
  generateLineaRollupClient,
  generateTransactionReceiptWithLogs,
} from "../../../utils/testing/helpers";
import { LineaRollup, LineaRollup__factory } from "../../../contracts/typechain";
import { EthersL2MessageServiceLogClient } from "../../linea/EthersL2MessageServiceLogClient";
import { EthersLineaRollupLogClient } from "../EthersLineaRollupLogClient";
import { MerkleTreeService } from "../MerkleTreeService";
import { LineaProvider, Provider } from "../../providers";

describe("MerkleTreeService", () => {
  let providerMock: MockProxy<Provider>;
  let l2ProviderMock: MockProxy<LineaProvider>;
  let walletMock: MockProxy<Wallet>;
  let lineaRollupMock: MockProxy<LineaRollup>;

  let merkleTreeService: MerkleTreeService;
  let lineaRollupLogClient: EthersLineaRollupLogClient;
  let l2MessageServiceLogClient: EthersL2MessageServiceLogClient;

  beforeEach(() => {
    providerMock = mock<Provider>();
    l2ProviderMock = mock<LineaProvider>();
    walletMock = mock<Wallet>();
    lineaRollupMock = mock<LineaRollup>();

    const clients = generateLineaRollupClient(
      providerMock,
      l2ProviderMock,
      TEST_CONTRACT_ADDRESS_1,
      TEST_CONTRACT_ADDRESS_2,
      "read-write",
      walletMock,
    );
    merkleTreeService = clients.merkleTreeService;
    l2MessageServiceLogClient = clients.l2MessageServiceLogClient;
    lineaRollupLogClient = clients.lineaRollupLogClient;
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
        .spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);
      jest.spyOn(LineaRollup__factory, "connect").mockReturnValueOnce(lineaRollupMock);

      await expect(merkleTreeService.getMessageProof(messageHash)).rejects.toThrow("Merkle tree build failed.");
    });
  });
});
