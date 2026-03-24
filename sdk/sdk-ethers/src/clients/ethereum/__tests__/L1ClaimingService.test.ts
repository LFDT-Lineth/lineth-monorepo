import { describe, it } from "@jest/globals";
import { ContractTransactionResponse, ethers } from "ethers";
import { MockProxy, mock, mockClear } from "jest-mock-extended";

import { ZERO_ADDRESS } from "../../../core/constants";
import {
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
  TEST_MESSAGE_HASH_2,
  TEST_TRANSACTION_HASH,
} from "../../../utils/testing/constants/common";
import {
  testL2MessagingBlockAnchoredEvent,
  testMessageSentEvent,
  testServiceVersionMigratedEvent,
} from "../../../utils/testing/constants/events";
import {
  generateHexString,
  generateL2MerkleTreeAddedLog,
  generateL2MessageServiceClient,
  generateL2MessagingBlockAnchoredLog,
  generateLineaRollupClient,
  generateMessage,
  generateTransactionReceipt,
  generateTransactionResponse,
} from "../../../utils/testing/helpers";
import { L2MessageServiceClient } from "../../linea";
import { EthersL2MessageServiceLogClient } from "../../linea/EthersL2MessageServiceLogClient";
import { LineaProvider, Provider } from "../../providers";
import { EthersLineaRollupLogClient } from "../EthersLineaRollupLogClient";
import { L1ClaimingService } from "../L1ClaimingService";
import { LineaRollupClient } from "../LineaRollupClient";

describe("L1ClaimingService", () => {
  let l1Provider: MockProxy<Provider>;
  let l2Provider: MockProxy<LineaProvider>;

  let l1ClaimingService: L1ClaimingService;
  let lineaRollupClient: LineaRollupClient;
  let l2MessageServiceClient: L2MessageServiceClient;
  let l2Client2MessageServiceLogClient: EthersL2MessageServiceLogClient;
  let l1ClientL2MessageServiceLogClient: EthersL2MessageServiceLogClient;
  let l1LogClient: EthersLineaRollupLogClient;

  beforeEach(() => {
    l1Provider = mock<Provider>();
    l2Provider = mock<LineaProvider>();

    const clients = generateLineaRollupClient(
      l1Provider,
      l2Provider,
      TEST_CONTRACT_ADDRESS_1,
      TEST_CONTRACT_ADDRESS_2,
      "read-only",
    );

    lineaRollupClient = clients.lineaRollupClient;
    l1LogClient = clients.lineaRollupLogClient;
    l1ClientL2MessageServiceLogClient = clients.l2MessageServiceLogClient;

    const l2Clients = generateL2MessageServiceClient(l2Provider, TEST_CONTRACT_ADDRESS_2, "read-only");
    l2MessageServiceClient = l2Clients.l2MessageServiceClient;
    l2Client2MessageServiceLogClient = l2Clients.l2MessageServiceLogClient;

    l1ClaimingService = new L1ClaimingService(
      lineaRollupClient,
      l2MessageServiceClient,
      l2Client2MessageServiceLogClient,
      "linea-sepolia",
    );
  });

  afterEach(() => {
    mockClear(l1Provider);
    mockClear(l2Provider);
    jest.clearAllMocks();
  });

  describe("getFinalizationMessagingInfo", () => {
    it("should throw an error when there are no logs in the transaction receipt", async () => {
      jest.spyOn(l1Provider, "getTransactionReceipt").mockResolvedValueOnce(
        generateTransactionReceipt({
          logs: [],
        }),
      );

      await expect(l1ClaimingService.getFinalizationMessagingInfo(TEST_TRANSACTION_HASH)).rejects.toThrow(
        `Transaction does not exist or no logs found in this transaction: ${TEST_TRANSACTION_HASH}.`,
      );
    });

    it("should throw an error when there are no L2MerkleRootAdded logs in the transaction receipt", async () => {
      jest.spyOn(l1Provider, "getTransactionReceipt").mockResolvedValueOnce(
        generateTransactionReceipt({
          logs: [generateL2MessagingBlockAnchoredLog(10n)],
        }),
      );

      await expect(l1ClaimingService.getFinalizationMessagingInfo(TEST_TRANSACTION_HASH)).rejects.toThrow(
        "No L2MerkleRootAdded events found in this transaction.",
      );
    });

    it("should throw an error when there are no L2MessagingBlocksAnchored logs in the transaction receipt", async () => {
      jest.spyOn(l1Provider, "getTransactionReceipt").mockResolvedValueOnce(
        generateTransactionReceipt({
          logs: [generateL2MerkleTreeAddedLog(generateHexString(32), 5)],
        }),
      );

      await expect(l1ClaimingService.getFinalizationMessagingInfo(TEST_TRANSACTION_HASH)).rejects.toThrow(
        "No L2MessagingBlocksAnchored events found in this transaction.",
      );
    });

    it("should return finalization messaging infos", async () => {
      const l2MerkleRoots = [generateHexString(32), generateHexString(32)];
      jest.spyOn(l1Provider, "getTransactionReceipt").mockResolvedValueOnce(
        generateTransactionReceipt({
          logs: [
            generateL2MerkleTreeAddedLog(l2MerkleRoots[0], 5),
            generateL2MerkleTreeAddedLog(l2MerkleRoots[1], 5),
            generateL2MessagingBlockAnchoredLog(10n),
            generateL2MessagingBlockAnchoredLog(15n),
            generateL2MessagingBlockAnchoredLog(20n),
            generateL2MessagingBlockAnchoredLog(25n),
          ],
        }),
      );

      expect(await l1ClaimingService.getFinalizationMessagingInfo(TEST_TRANSACTION_HASH)).toStrictEqual({
        l2MessagingBlocksRange: {
          startingBlock: 10,
          endBlock: 25,
        },
        l2MerkleRoots,
        treeDepth: 5,
      });
    });
  });

  describe("getL2MessageHashesInBlockRange", () => {
    it("should throw an error when there are no MessageSent events in the block range", async () => {
      jest.spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByBlockRange").mockResolvedValueOnce([]);

      await expect(l1ClaimingService.getL2MessageHashesInBlockRange(10, 25)).rejects.toThrow(
        "No MessageSent events found in this block range on L2.",
      );
    });

    it("should return messages hashes in the block range", async () => {
      jest.spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByBlockRange").mockResolvedValueOnce([
        testMessageSentEvent,
        {
          ...testMessageSentEvent,
          messageHash: TEST_MESSAGE_HASH_2,
        },
      ]);

      expect(await l1ClaimingService.getL2MessageHashesInBlockRange(10, 25)).toStrictEqual([
        TEST_MESSAGE_HASH,
        TEST_MESSAGE_HASH_2,
      ]);
    });
  });

  describe("getMessageSiblings", () => {
    it("should throw an error when the message hash is not found in the message hashes array", async () => {
      expect(() =>
        l1ClaimingService.getMessageSiblings(generateHexString(32), [TEST_MESSAGE_HASH, TEST_MESSAGE_HASH_2], 5),
      ).toThrow("Message hash not found in messages.");
    });

    it.each([
      {
        messageHashes: Array.from({ length: 32 }, () => generateHexString(32)),
        messageIndex: 2,
        expectedIndexRange: {
          start: 0,
          end: 32,
          numberOfZeroHashes: 0,
        },
      },
      {
        messageHashes: Array.from({ length: 35 }, () => generateHexString(32)),
        messageIndex: 33,
        expectedIndexRange: {
          start: 32,
          end: 35,
          numberOfZeroHashes: 29,
        },
      },
      {
        messageHashes: Array.from({ length: 100 }, () => generateHexString(32)),
        messageIndex: 90,
        expectedIndexRange: {
          start: 64,
          end: 96,
          numberOfZeroHashes: 0,
        },
      },
    ])(
      "should return message siblings with message index $messageIndex and $messageHashes.length messages",
      ({ messageHashes, messageIndex, expectedIndexRange }) => {
        const messageHash = messageHashes[messageIndex];
        const zeroHashes =
          expectedIndexRange.numberOfZeroHashes > 0
            ? Array(expectedIndexRange.numberOfZeroHashes).fill(ethers.ZeroHash)
            : [];
        expect(l1ClaimingService.getMessageSiblings(messageHash, messageHashes, 5)).toStrictEqual([
          ...messageHashes.slice(expectedIndexRange.start, expectedIndexRange.end),
          ...zeroHashes,
        ]);
      },
    );
  });

  describe("isMessageSentAfterMigrationBlock", () => {
    it("should throw an error when no MessageSent event has been emitted", async () => {
      jest.spyOn(l2Client2MessageServiceLogClient, "getMessageSentEventsByMessageHash").mockResolvedValueOnce([]);
      await expect(l1ClaimingService.isMessageSentAfterMigrationBlock(TEST_MESSAGE_HASH, 50)).rejects.toThrow(
        `Message hash does not exist on L2. Message hash: ${TEST_MESSAGE_HASH}`,
      );
    });

    it("should return false when the message has been sent before the service version migration", async () => {
      jest
        .spyOn(l2Client2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValueOnce([{ ...testMessageSentEvent, blockNumber: 50 }]);
      expect(await l1ClaimingService.isMessageSentAfterMigrationBlock(TEST_MESSAGE_HASH, 51)).toBeFalsy();
    });

    it("should return true when the message has been sent after the service version migration", async () => {
      jest
        .spyOn(l2Client2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValueOnce([{ ...testMessageSentEvent, blockNumber: 52 }]);
      expect(await l1ClaimingService.isMessageSentAfterMigrationBlock(TEST_MESSAGE_HASH, 49)).toBeTruthy();
    });
  });

  describe("findMigrationBlock", () => {
    it("should return null when no ServiceVersionMigrated event has been emitted", async () => {
      jest.spyOn(l2Client2MessageServiceLogClient, "getServiceVersionMigratedEvents").mockResolvedValueOnce([]);
      expect(await l1ClaimingService.findMigrationBlock()).toBeNull();
    });

    it("should return the migration block without using cache", async () => {
      const serviceVersionMigratedEventSpy = jest
        .spyOn(l2Client2MessageServiceLogClient, "getServiceVersionMigratedEvents")
        .mockResolvedValueOnce([testServiceVersionMigratedEvent]);

      expect(await l1ClaimingService.findMigrationBlock()).toEqual(51);
      expect(serviceVersionMigratedEventSpy).toHaveBeenCalledTimes(1);
    });

    it("should return the migration block using cache", async () => {
      const serviceVersionMigratedEventSpy = jest
        .spyOn(l2Client2MessageServiceLogClient, "getServiceVersionMigratedEvents")
        .mockResolvedValueOnce([testServiceVersionMigratedEvent]);

      // without using cache
      await l1ClaimingService.findMigrationBlock();
      // using cache
      const migrationBlock = await l1ClaimingService.findMigrationBlock();

      expect(migrationBlock).toEqual(51);
      expect(serviceVersionMigratedEventSpy).toHaveBeenCalledTimes(1);
    });
  });

  describe("getMessageProof", () => {
    it("should throw an error when there is no MessageSent event on L2 for the message hash", async () => {
      jest.spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByMessageHash").mockResolvedValueOnce([]);

      await expect(l1ClaimingService.getMessageProof(TEST_MESSAGE_HASH)).rejects.toThrow(
        `Message hash does not exist on L2. Message hash: ${TEST_MESSAGE_HASH}`,
      );
    });

    it("should throw an error when there is no L2MessagingBlockAnchored event on L1 for the message sent event block number", async () => {
      jest
        .spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValueOnce([testMessageSentEvent]);
      jest.spyOn(l1LogClient, "getL2MessagingBlockAnchoredEvents").mockResolvedValueOnce([]);

      await expect(l1ClaimingService.getMessageProof(TEST_MESSAGE_HASH)).rejects.toThrow(
        `L2 block number ${testMessageSentEvent.blockNumber} has not been finalized on L1.`,
      );
    });

    it("should throw an error when the built tree root is not included in the finalization l2MerkleRoots array", async () => {
      jest
        .spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValueOnce([testMessageSentEvent])
        .mockResolvedValueOnce([
          testMessageSentEvent,
          {
            ...testMessageSentEvent,
            messageHash: TEST_MESSAGE_HASH_2,
          },
        ]);
      jest
        .spyOn(l1LogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValueOnce([testL2MessagingBlockAnchoredEvent]);
      jest
        .spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByBlockRange")
        .mockResolvedValueOnce([testMessageSentEvent]);

      const l2MerkleRoots = [generateHexString(32)];

      jest.spyOn(l1Provider, "getTransactionReceipt").mockResolvedValueOnce(
        generateTransactionReceipt({
          logs: [
            generateL2MerkleTreeAddedLog(l2MerkleRoots[0], 5),
            generateL2MessagingBlockAnchoredLog(10n),
            generateL2MessagingBlockAnchoredLog(15n),
            generateL2MessagingBlockAnchoredLog(25n),
            generateL2MessagingBlockAnchoredLog(51n),
          ],
        }),
      );
      await expect(l1ClaimingService.getMessageProof(TEST_MESSAGE_HASH)).rejects.toThrow("Merkle tree build failed.");
    });

    it("should return the message proof", async () => {
      jest
        .spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValueOnce([testMessageSentEvent])
        .mockResolvedValueOnce([
          testMessageSentEvent,
          {
            ...testMessageSentEvent,
            messageHash: TEST_MESSAGE_HASH_2,
          },
        ]);
      jest
        .spyOn(l1LogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValueOnce([testL2MessagingBlockAnchoredEvent]);
      jest
        .spyOn(l1ClientL2MessageServiceLogClient, "getMessageSentEventsByBlockRange")
        .mockResolvedValueOnce([testMessageSentEvent]);

      const l2MerkleRoots = ["0xfc3dfe7470d41465e77e7c929170578b14a066a2272c2469b60162c5282e05a6"];

      jest.spyOn(l1Provider, "getTransactionReceipt").mockResolvedValueOnce(
        generateTransactionReceipt({
          logs: [
            generateL2MerkleTreeAddedLog(l2MerkleRoots[0], 5),
            generateL2MessagingBlockAnchoredLog(10n),
            generateL2MessagingBlockAnchoredLog(15n),
            generateL2MessagingBlockAnchoredLog(25n),
            generateL2MessagingBlockAnchoredLog(51n),
          ],
        }),
      );
      expect(await l1ClaimingService.getMessageProof(TEST_MESSAGE_HASH)).toStrictEqual({
        leafIndex: 0,
        proof: [
          "0x0000000000000000000000000000000000000000000000000000000000000000",
          "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
          "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
          "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
          "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        ],
        root: "0xfc3dfe7470d41465e77e7c929170578b14a066a2272c2469b60162c5282e05a6",
      });
    });
  });

  describe("isClaimingNeedingProof", () => {
    it("should return false when there is migration block event", async () => {
      jest.spyOn(l2Client2MessageServiceLogClient, "getServiceVersionMigratedEvents").mockResolvedValueOnce([]);

      expect(await l1ClaimingService.isClaimingNeedingProof(TEST_MESSAGE_HASH)).toBeFalsy();
    });

    it("should return true when message has been sent after migration block", async () => {
      jest
        .spyOn(l2Client2MessageServiceLogClient, "getServiceVersionMigratedEvents")
        .mockResolvedValueOnce([testServiceVersionMigratedEvent]);
      jest
        .spyOn(l2Client2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValueOnce([testMessageSentEvent]);

      expect(await l1ClaimingService.isClaimingNeedingProof(TEST_MESSAGE_HASH)).toBeTruthy();
    });

    it("should return false when message has been sent before migration block", async () => {
      jest
        .spyOn(l2Client2MessageServiceLogClient, "getServiceVersionMigratedEvents")
        .mockResolvedValueOnce([testServiceVersionMigratedEvent]);
      jest
        .spyOn(l2Client2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValueOnce([{ ...testMessageSentEvent, blockNumber: 1 }]);

      expect(await l1ClaimingService.isClaimingNeedingProof(TEST_MESSAGE_HASH)).toBeFalsy();
    });
  });

  describe("estimateClaimMessageGas", () => {
    it("should use the old estimate gas method when proof is not needed", async () => {
      jest.spyOn(l1ClaimingService, "isClaimingNeedingProof").mockResolvedValueOnce(false);
      const estimateClaimGasSpy = jest
        .spyOn(lineaRollupClient, "estimateClaimWithoutProofGas")
        .mockResolvedValue(50_000n);
      const estimateClaimWithProofGasSpy = jest.spyOn(lineaRollupClient, "estimateClaimGas");

      const { messageSender, destination, fee, value, messageNonce, calldata, messageHash } = generateMessage();
      await l1ClaimingService.estimateClaimMessageGas({
        messageSender,
        destination,
        fee,
        value,
        messageNonce,
        calldata,
        messageHash,
        feeRecipient: ZERO_ADDRESS,
        blockNumber: 0,
        logIndex: 0,
        contractAddress: "",
        transactionHash: "",
      });

      expect(estimateClaimGasSpy).toHaveBeenCalledTimes(1);
      expect(estimateClaimGasSpy).toHaveBeenCalledWith(
        {
          messageSender,
          destination,
          fee,
          value,
          messageNonce,
          calldata,
          messageHash,
          feeRecipient: ZERO_ADDRESS,
          blockNumber: 0,
          logIndex: 0,
          contractAddress: "",
          transactionHash: "",
        },
        {},
      );
      expect(estimateClaimWithProofGasSpy).toHaveBeenCalledTimes(0);
    });

    it("should use the new estimate gas method when proof is needed", async () => {
      jest.spyOn(l1ClaimingService, "isClaimingNeedingProof").mockResolvedValueOnce(true);
      const estimateClaimWithoutProofGasSpy = jest.spyOn(lineaRollupClient, "estimateClaimWithoutProofGas");
      const estimateClaimWithProofGasSpy = jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValue(50_000n);

      const { messageSender, destination, fee, value, messageNonce, calldata, messageHash } = generateMessage();
      await l1ClaimingService.estimateClaimMessageGas({
        messageSender,
        destination,
        fee,
        value,
        messageNonce,
        calldata,
        messageHash,
        feeRecipient: ZERO_ADDRESS,
        blockNumber: 0,
        logIndex: 0,
        contractAddress: "",
        transactionHash: "",
      });

      expect(estimateClaimWithoutProofGasSpy).toHaveBeenCalledTimes(0);
      expect(estimateClaimWithProofGasSpy).toHaveBeenCalledTimes(1);
      expect(estimateClaimWithProofGasSpy).toHaveBeenCalledWith(
        {
          messageSender,
          destination,
          fee,
          value,
          messageNonce,
          calldata,
          messageHash,
          feeRecipient: ZERO_ADDRESS,
          blockNumber: 0,
          logIndex: 0,
          contractAddress: "",
          transactionHash: "",
        },
        {},
      );
    });
  });

  describe("claimMessage", () => {
    it("should use the old claimMessage method when proof is not needed", async () => {
      jest.spyOn(l1ClaimingService, "isClaimingNeedingProof").mockResolvedValueOnce(false);
      const claimWithoutProofSpy = jest
        .spyOn(lineaRollupClient, "claimWithoutProof")
        .mockResolvedValue(generateTransactionResponse() as ContractTransactionResponse);
      const claimWithProofSpy = jest.spyOn(lineaRollupClient, "claim");

      const { messageSender, destination, fee, value, messageNonce, calldata, messageHash } = generateMessage();
      await l1ClaimingService.claimMessage({
        messageSender,
        destination,
        fee,
        value,
        messageNonce,
        calldata,
        messageHash,
        feeRecipient: ZERO_ADDRESS,
        blockNumber: 0,
        logIndex: 0,
        contractAddress: "",
        transactionHash: "",
      });

      expect(claimWithoutProofSpy).toHaveBeenCalledTimes(1);
      expect(claimWithoutProofSpy).toHaveBeenCalledWith(
        {
          messageSender,
          destination,
          fee,
          value,
          messageNonce,
          calldata,
          messageHash,
          feeRecipient: ZERO_ADDRESS,
          blockNumber: 0,
          logIndex: 0,
          contractAddress: "",
          transactionHash: "",
        },
        {},
      );
      expect(claimWithProofSpy).toHaveBeenCalledTimes(0);
    });

    it("should use the new claimMessageWithProof method when proof is needed", async () => {
      jest.spyOn(l1ClaimingService, "isClaimingNeedingProof").mockResolvedValueOnce(true);
      const claimWithoutProofSpy = jest.spyOn(lineaRollupClient, "claimWithoutProof");
      const claimWithProofSpy = jest
        .spyOn(lineaRollupClient, "claim")
        .mockResolvedValue(generateTransactionResponse() as ContractTransactionResponse);

      const { messageSender, destination, fee, value, messageNonce, calldata, messageHash } = generateMessage();
      await l1ClaimingService.claimMessage({
        messageSender,
        destination,
        fee,
        value,
        messageNonce,
        calldata,
        messageHash,
        feeRecipient: ZERO_ADDRESS,
        blockNumber: 0,
        logIndex: 0,
        contractAddress: "",
        transactionHash: "",
      });

      expect(claimWithoutProofSpy).toHaveBeenCalledTimes(0);
      expect(claimWithProofSpy).toHaveBeenCalledTimes(1);
      expect(claimWithProofSpy).toHaveBeenCalledWith(
        {
          messageSender,
          destination,
          fee,
          value,
          messageNonce,
          calldata,
          messageHash,
          feeRecipient: ZERO_ADDRESS,
          blockNumber: 0,
          logIndex: 0,
          contractAddress: "",
          transactionHash: "",
        },
        {},
      );
    });
  });
});
