import { describe, afterEach, it, expect, beforeEach } from "@jest/globals";
import { ContractTransactionResponse, Wallet } from "ethers";
import { MockProxy, mock, mockClear, mockDeep } from "jest-mock-extended";

import { LineaRollup, LineaRollup__factory } from "../../../contracts/typechain";
import { ZERO_ADDRESS } from "../../../core/constants";
import { OnChainMessageStatus } from "../../../core/enums/message";
import { BaseError, makeBaseError } from "../../../core/errors";
import {
  TEST_MESSAGE_HASH,
  TEST_CONTRACT_ADDRESS_1,
  TEST_TRANSACTION_HASH,
  TEST_ADDRESS_2,
  TEST_MERKLE_ROOT,
  TEST_CONTRACT_ADDRESS_2,
  TEST_ADDRESS_1,
  DEFAULT_MAX_FEE_PER_GAS,
} from "../../../utils/testing/constants/common";
import {
  testMessageSentEvent,
  testMessageClaimedEvent,
  testL2MessagingBlockAnchoredEvent,
} from "../../../utils/testing/constants/events";
import {
  generateL2MerkleTreeAddedLog,
  generateL2MessagingBlockAnchoredLog,
  generateLineaRollupClient,
  generateMessage,
  generateTransactionReceipt,
  generateTransactionReceiptWithLogs,
  generateTransactionResponse,
  mockProperty,
} from "../../../utils/testing/helpers";
import { DefaultGasProvider } from "../../gas/DefaultGasProvider";
import { EthersL2MessageServiceLogClient } from "../../linea/EthersL2MessageServiceLogClient";
import { LineaProvider, Provider } from "../../providers";
import { EthersLineaRollupLogClient } from "../EthersLineaRollupLogClient";
import { LineaRollupClient } from "../LineaRollupClient";

describe("TestLineaRollupClient", () => {
  let providerMock: MockProxy<Provider>;
  let l2ProviderMock: MockProxy<LineaProvider>;
  let walletMock: MockProxy<Wallet>;
  let lineaRollupMock: MockProxy<LineaRollup>;

  let lineaRollupClient: LineaRollupClient;
  let lineaRollupLogClient: EthersLineaRollupLogClient;
  let l2MessageServiceLogClient: EthersL2MessageServiceLogClient;
  let gasFeeProvider: DefaultGasProvider;

  beforeEach(() => {
    providerMock = mock<Provider>();
    l2ProviderMock = mock<LineaProvider>();
    walletMock = mock<Wallet>();
    lineaRollupMock = mockDeep<LineaRollup>();
    jest.spyOn(LineaRollup__factory, "connect").mockReturnValue(lineaRollupMock);
    walletMock.getAddress.mockResolvedValue(TEST_ADDRESS_1);
    lineaRollupMock.getAddress.mockResolvedValue(TEST_CONTRACT_ADDRESS_1);

    const clients = generateLineaRollupClient(
      providerMock,
      l2ProviderMock,
      TEST_CONTRACT_ADDRESS_1,
      TEST_CONTRACT_ADDRESS_2,
      "read-write",
      walletMock,
    );
    lineaRollupClient = clients.lineaRollupClient;
    lineaRollupLogClient = clients.lineaRollupLogClient;
    l2MessageServiceLogClient = clients.l2MessageServiceLogClient;
    gasFeeProvider = clients.gasProvider;
  });

  afterEach(() => {
    mockClear(providerMock);
    mockClear(l2ProviderMock);
    mockClear(walletMock);
    mockClear(lineaRollupMock);
    jest.clearAllMocks();
  });

  describe("constructor", () => {
    it("should throw an error when mode = 'read-write' and this.signer is undefined", async () => {
      expect(
        () =>
          generateLineaRollupClient(
            providerMock,
            l2ProviderMock,
            TEST_CONTRACT_ADDRESS_1,
            TEST_CONTRACT_ADDRESS_2,
            "read-write",
          ).lineaRollupClient,
      ).toThrow(new BaseError("Please provide a signer."));
    });
  });

  describe("getMessageStatusUsingMessageHash", () => {
    it("should return UNKNOWN when on chain message status === 0 and no claimed event was found", async () => {
      jest.spyOn(lineaRollupMock, "inboxL2L1MessageStatus").mockResolvedValue(0n);
      jest.spyOn(lineaRollupLogClient, "getMessageClaimedEvents").mockResolvedValue([]);

      const messageStatus = await lineaRollupClient.getMessageStatusUsingMessageHash(TEST_MESSAGE_HASH);

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.UNKNOWN);
    });

    it("should return CLAIMABLE when on chain message status === 1", async () => {
      jest.spyOn(lineaRollupMock, "inboxL2L1MessageStatus").mockResolvedValue(1n);

      const messageStatus = await lineaRollupClient.getMessageStatusUsingMessageHash(TEST_MESSAGE_HASH);

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMABLE);
    });

    it("should return CLAIMED when on chain message status === 0 and the claimed event was found", async () => {
      jest.spyOn(lineaRollupMock, "inboxL2L1MessageStatus").mockResolvedValue(0n);
      jest.spyOn(lineaRollupLogClient, "getMessageClaimedEvents").mockResolvedValue([testMessageClaimedEvent]);

      const messageStatus = await lineaRollupClient.getMessageStatusUsingMessageHash(TEST_MESSAGE_HASH);

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMED);
    });
  });

  describe("getMessageStatus", () => {
    it("should return UNKNOWN when l2MessagingBlockAnchoredEvent is absent and isMeessageClaimed return false", async () => {
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents").mockResolvedValue([]);
      jest.spyOn(lineaRollupMock, "isMessageClaimed").mockResolvedValue(false);

      const messageStatus = await lineaRollupClient.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.UNKNOWN);
    });

    it("should return CLAIMABLE when l2MessagingBlockAnchoredEvent is present and isMessageClaimed return false", async () => {
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest
        .spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(lineaRollupMock, "isMessageClaimed").mockResolvedValue(false);

      const messageStatus = await lineaRollupClient.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMABLE);
    });

    it("should return CLAIMED when isMessageClaimed return true", async () => {
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents").mockResolvedValue([]);
      jest.spyOn(lineaRollupMock, "isMessageClaimed").mockResolvedValue(true);

      const messageStatus = await lineaRollupClient.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMED);
    });
  });

  describe("getMessageStatusUsingMerkleTree", () => {
    it("should throw error when the corresponding message sent event was not found on L2", async () => {
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash").mockResolvedValue([]);

      await expect(
        lineaRollupClient.getMessageStatusUsingMerkleTree({ messageHash: TEST_MESSAGE_HASH }),
      ).rejects.toThrow(new BaseError(`Message hash does not exist on L2. Message hash: ${TEST_MESSAGE_HASH}`));
    });

    it("should return UNKNOWN when l2MessagingBlockAnchoredEvent is absent and isMeessageClaimed return false", async () => {
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents").mockResolvedValue([]);
      jest.spyOn(lineaRollupMock, "isMessageClaimed").mockResolvedValue(false);

      const messageStatus = await lineaRollupClient.getMessageStatusUsingMerkleTree({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.UNKNOWN);
    });

    it("should return CLAIMABLE when l2MessagingBlockAnchoredEvent is present and isMessageClaimed return false", async () => {
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest
        .spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(lineaRollupMock, "isMessageClaimed").mockResolvedValue(false);

      const messageStatus = await lineaRollupClient.getMessageStatusUsingMerkleTree({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMABLE);
    });

    it("should return CLAIMED when isMessageClaimed return true", async () => {
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents").mockResolvedValue([]);
      jest.spyOn(lineaRollupMock, "isMessageClaimed").mockResolvedValue(true);

      const messageStatus = await lineaRollupClient.getMessageStatusUsingMerkleTree({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMED);
    });
  });

  describe("estimateClaimWithoutProofGasFees", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      const lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-only",
      ).lineaRollupClient;

      const message = generateMessage();
      await expect(lineaRollupClient.estimateClaimWithoutProofGas(message)).rejects.toThrow(
        new BaseError("'EstimateClaimGas' function not callable using readOnly mode."),
      );
    });

    it("should throw a GasEstimationError when the gas estimation failed", async () => {
      const message = generateMessage();
      jest.spyOn(gasFeeProvider, "getGasFees").mockRejectedValue(new Error("Gas fees estimation failed").message);

      await expect(lineaRollupClient.estimateClaimWithoutProofGas(message)).rejects.toThrow(
        makeBaseError("Gas fees estimation failed", message),
      );
    });

    it("should set feeRecipient === ZeroAddress when feeRecipient param is undefined", async () => {
      const message = generateMessage();
      const estimatedGasLimit = 50_000n;
      mockProperty(lineaRollupMock, "claimMessage", {
        estimateGas: jest.fn().mockResolvedValueOnce(estimatedGasLimit),
      } as any);

      const gasFeesSpy = jest.spyOn(gasFeeProvider, "getGasFees").mockResolvedValue({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });
      const claimMessageSpy = jest.spyOn(lineaRollupMock.claimMessage, "estimateGas");

      const estimatedGas = await lineaRollupClient.estimateClaimWithoutProofGas(message);

      expect(estimatedGas).toStrictEqual(estimatedGasLimit);
      expect(gasFeesSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledWith(
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        ZERO_ADDRESS,
        message.calldata,
        message.messageNonce,
        {
          maxFeePerGas: 100000000000n,
          maxPriorityFeePerGas: 100000000000n,
        },
      );
    });

    it("should return estimated gas limit for the claim message transaction", async () => {
      const message = generateMessage();
      const estimatedGasLimit = 50_000n;

      mockProperty(lineaRollupMock, "claimMessage", {
        estimateGas: jest.fn().mockResolvedValueOnce(estimatedGasLimit),
      } as any);

      const claimMessageSpy = jest.spyOn(lineaRollupMock.claimMessage, "estimateGas");

      const gasFeesSpy = jest.spyOn(gasFeeProvider, "getGasFees").mockResolvedValue({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });

      const estimateClaimGas = await lineaRollupClient.estimateClaimWithoutProofGas({
        ...message,
        feeRecipient: TEST_ADDRESS_2,
      });

      expect(estimateClaimGas).toStrictEqual(estimatedGasLimit);
      expect(gasFeesSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledWith(
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        TEST_ADDRESS_2,
        message.calldata,
        message.messageNonce,
        {
          maxFeePerGas: 100000000000n,
          maxPriorityFeePerGas: 100000000000n,
        },
      );
    });
  });
  //   it("should throw an error when mode = 'read-only'", async () => {
  //     lineaRollupClient = new LineaRollupClient(
  //       providerMock,
  //       TEST_CONTRACT_ADDRESS_1,
  //       lineaRollupLogClientMock,
  //       l2MessageServiceLogClientMock,
  //       "read-only",
  //       walletMock,
  //     );
  //     const message = generateMessage();
  //     await expect(
  //       lineaRollupClient.estimateClaimWithProofGas({
  //         ...message,
  //         leafIndex: 0,
  //         merkleRoot: TEST_MERKLE_ROOT,
  //         proof: [
  //           "0x0000000000000000000000000000000000000000000000000000000000000000",
  //           "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
  //           "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
  //           "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
  //           "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
  //         ],
  //       }),
  //     ).rejects.toThrow("'EstimateClaimWithProofGas' function not callable using readOnly mode.");
  //   });

  //   it("should throw GasEstimationError if estimateGas throws error", async () => {
  //     const message = generateMessage();
  //     mockProperty(lineaRollupMock, "claimMessageWithProof", {
  //       estimateGas: jest.fn().mockRejectedValueOnce(new Error("Failed to estimate gas")),
  //       // eslint-disable-next-line @typescript-eslint/no-explicit-any
  //     } as any);
  //     jest.spyOn(LineaRollup__factory, "connect").mockReturnValueOnce(lineaRollupMock);
  //     lineaRollupClient = new LineaRollupClient(
  //       providerMock,
  //       TEST_CONTRACT_ADDRESS_1,
  //       lineaRollupLogClientMock,
  //       l2MessageServiceLogClientMock,
  //       "read-write",
  //       walletMock,
  //       1000000000n,
  //       undefined,
  //       true,
  //     );
  //     await expect(
  //       lineaRollupClient.estimateClaimWithProofGas({
  //         ...message,
  //         leafIndex: 0,
  //         merkleRoot: TEST_MERKLE_ROOT,
  //         proof: [
  //           "0x0000000000000000000000000000000000000000000000000000000000000000",
  //           "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
  //           "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
  //           "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
  //           "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
  //         ],
  //       }),
  //     ).rejects.toThrow("Failed to estimate gas");
  //   });
  // });

  describe("estimateClaimGas", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-only",
        walletMock,
      ).lineaRollupClient;

      const message = generateMessage();
      await expect(lineaRollupClient.estimateClaimGas(message)).rejects.toThrow(
        "'EstimateClaimGasFees' function not callable using readOnly mode.",
      );
    });

    it("should throw a GasEstimationError when the message hash does not exist on L2", async () => {
      const message = generateMessage();
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash").mockResolvedValue([]);

      await expect(lineaRollupClient.estimateClaimGas(message)).rejects.toThrow(
        `Message hash does not exist on L2. Message hash: ${TEST_MESSAGE_HASH}`,
      );
    });

    it("should throw a GasEstimationError when the L2 block number has not been finalized on L1", async () => {
      const message = generateMessage();
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents").mockResolvedValue([]);

      await expect(lineaRollupClient.estimateClaimGas(message)).rejects.toThrow(
        "L2 block number 51 has not been finalized on L1",
      );
    });

    it("should throw a GasEstimationError when finalization transaction does not exist", async () => {
      const message = generateMessage();
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest
        .spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(null);

      await expect(lineaRollupClient.estimateClaimGas(message)).rejects.toThrow(
        `Transaction does not exist or no logs found in this transaction: ${TEST_TRANSACTION_HASH}.`,
      );
    });

    it("should throw a GasEstimationError when no related event logs were found", async () => {
      const message = generateMessage();
      const transactionReceipt = generateTransactionReceipt();
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest
        .spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);

      await expect(lineaRollupClient.estimateClaimGas(message)).rejects.toThrow(
        "No L2MerkleRootAdded events found in this transaction.",
      );
    });

    it("should throw a GasEstimationError when no L2MessagingBlocksAnchored event logs were found", async () => {
      const message = generateMessage();
      const transactionReceipt = generateTransactionReceiptWithLogs(undefined, [
        generateL2MerkleTreeAddedLog(TEST_TRANSACTION_HASH, 5),
      ]);
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest
        .spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);

      await expect(lineaRollupClient.estimateClaimGas(message)).rejects.toThrow(
        "No L2MessagingBlocksAnchored events found in this transaction.",
      );
    });

    it("should throw a GasEstimationError when no MessageSent events found in the given block range on L2", async () => {
      const message = generateMessage();
      const transactionReceipt = generateTransactionReceiptWithLogs(undefined, [
        generateL2MerkleTreeAddedLog(TEST_TRANSACTION_HASH, 5),
        generateL2MessagingBlockAnchoredLog(10n),
      ]);
      jest
        .spyOn(l2MessageServiceLogClient, "getMessageSentEventsByMessageHash")
        .mockResolvedValue([testMessageSentEvent]);
      jest.spyOn(l2MessageServiceLogClient, "getMessageSentEventsByBlockRange").mockResolvedValue([]);
      jest
        .spyOn(lineaRollupLogClient, "getL2MessagingBlockAnchoredEvents")
        .mockResolvedValue([testL2MessagingBlockAnchoredEvent]);
      jest.spyOn(providerMock, "getTransactionReceipt").mockResolvedValue(transactionReceipt);

      await expect(lineaRollupClient.estimateClaimGas(message)).rejects.toThrow();
    });

    it("should return estimated gas limit if all the relevant event logs were found", async () => {
      const message = generateMessage();
      const transactionReceipt = generateTransactionReceiptWithLogs(undefined, [
        generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, 5),
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

      // const transactionData = LineaRollup__factory.createInterface().encodeFunctionData("claimMessageWithProof", [
      //   {
      //     from: message.messageSender,
      //     to: message.destination,
      //     fee: message.fee,
      //     value: message.value,
      //     feeRecipient: ZERO_ADDRESS,
      //     data: message.calldata,
      //     messageNumber: message.messageNonce,
      //     leafIndex: 0,
      //     merkleRoot: TEST_MERKLE_ROOT,
      //     proof: [
      //       "0x0000000000000000000000000000000000000000000000000000000000000000",
      //       "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
      //       "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
      //       "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
      //       "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
      //     ],
      //   },
      // ]);

      const estimatedGasLimit = 50_000n;
      mockProperty(lineaRollupMock, "claimMessageWithProof", {
        estimateGas: jest.fn().mockResolvedValueOnce(estimatedGasLimit),
      } as any);
      mockProperty(lineaRollupMock, "interface", {
        parseLog: jest
          .fn()
          .mockReturnValueOnce({
            args: { treeDepth: 5, l2MerkleRoot: TEST_MERKLE_ROOT },
          } as any)
          .mockReturnValueOnce({
            args: { l2Block: 10n },
          } as any),
      } as any);

      const gasFeesSpy = jest.spyOn(gasFeeProvider, "getGasFees").mockResolvedValue({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });

      const claimMessageWithProofSpy = jest.spyOn(lineaRollupMock.claimMessageWithProof, "estimateGas");

      const estimatedClaimGas = await lineaRollupClient.estimateClaimGas(message);

      expect(estimatedClaimGas).toStrictEqual(estimatedGasLimit);
      expect(gasFeesSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageWithProofSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageWithProofSpy).toHaveBeenCalledWith(
        {
          from: message.messageSender,
          to: message.destination,
          fee: message.fee,
          value: message.value,
          feeRecipient: ZERO_ADDRESS,
          data: message.calldata,
          messageNumber: message.messageNonce,
          leafIndex: 0,
          merkleRoot: TEST_MERKLE_ROOT,
          proof: [
            "0x0000000000000000000000000000000000000000000000000000000000000000",
            "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
            "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
            "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
            "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
          ],
        },
        {
          maxFeePerGas: 100000000000n,
          maxPriorityFeePerGas: 100000000000n,
        },
      );
    });
  });

  describe("claimWithoutProof", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-only",
        walletMock,
      ).lineaRollupClient;

      const message = generateMessage();
      await expect(lineaRollupClient.claimWithoutProof(message)).rejects.toThrow(
        new Error("'claim' function not callable using readOnly mode."),
      );
    });

    it("should set feeRecipient === ZeroAddress when feeRecipient param is undefined", async () => {
      const message = generateMessage();
      const txResponse = generateTransactionResponse();
      jest.spyOn(lineaRollupMock, "claimMessage").mockResolvedValue(txResponse as ContractTransactionResponse);
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-write",
        walletMock,
        {
          maxFeePerGasCap: 500000000n,
          enforceMaxGasFee: true,
        },
      ).lineaRollupClient;

      const claimMessageSpy = jest.spyOn(lineaRollupMock, "claimMessage");

      await lineaRollupClient.claimWithoutProof(message);

      expect(txResponse).toStrictEqual(txResponse);
      expect(claimMessageSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledWith(
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        ZERO_ADDRESS,
        message.calldata,
        message.messageNonce,
        {
          maxPriorityFeePerGas: 500000000n,
          maxFeePerGas: 500000000n,
        },
      );
    });

    it("should return executed claim message transaction", async () => {
      const message = generateMessage();
      const txResponse = generateTransactionResponse();
      jest.spyOn(lineaRollupMock, "claimMessage").mockResolvedValue(txResponse as ContractTransactionResponse);
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-write",
        walletMock,
        {
          maxFeePerGasCap: 500000000n,
          enforceMaxGasFee: true,
        },
      ).lineaRollupClient;

      const claimMessageSpy = jest.spyOn(lineaRollupMock, "claimMessage");

      const txResponseReturned = await lineaRollupClient.claimWithoutProof({
        ...message,
        feeRecipient: TEST_ADDRESS_2,
      });

      expect(txResponseReturned).toStrictEqual(txResponse);
      expect(claimMessageSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledWith(
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        TEST_ADDRESS_2,
        message.calldata,
        message.messageNonce,
        {
          maxPriorityFeePerGas: 500000000n,
          maxFeePerGas: 500000000n,
        },
      );
    });
  });

  describe("claim", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-only",
        walletMock,
      ).lineaRollupClient;

      const message = generateMessage();
      await expect(lineaRollupClient.claim(message)).rejects.toThrow(
        new Error("'claim' function not callable using readOnly mode."),
      );
    });

    it("should return executed claim message transaction", async () => {
      const message = generateMessage();
      const txResponse = generateTransactionResponse();
      const transactionReceipt = generateTransactionReceiptWithLogs(undefined, [
        generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, 5),
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

      mockProperty(lineaRollupMock, "interface", {
        parseLog: jest
          .fn()
          .mockReturnValueOnce({
            args: { treeDepth: 5, l2MerkleRoot: TEST_MERKLE_ROOT },
          } as any)
          .mockReturnValueOnce({
            args: { l2Block: 10n },
          } as any),
      } as any);
      jest.spyOn(gasFeeProvider, "getGasFees").mockResolvedValue({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });

      const claimMessageWithProofSpy = jest
        .spyOn(lineaRollupMock, "claimMessageWithProof")
        .mockResolvedValue(txResponse as ContractTransactionResponse);

      const txResponseReturned = await lineaRollupClient.claim(message);

      expect(txResponseReturned).toStrictEqual(txResponse);
      expect(claimMessageWithProofSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageWithProofSpy).toHaveBeenCalledWith(
        {
          from: message.messageSender,
          to: message.destination,
          fee: message.fee,
          value: message.value,
          feeRecipient: ZERO_ADDRESS,
          data: message.calldata,
          messageNumber: message.messageNonce,
          leafIndex: 0,
          merkleRoot: TEST_MERKLE_ROOT,
          proof: [
            "0x0000000000000000000000000000000000000000000000000000000000000000",
            "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
            "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
            "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
            "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
          ],
        },
        {
          maxFeePerGas: 100000000000n,
          maxPriorityFeePerGas: 100000000000n,
        },
      );
    });
  });

  describe("retryTransactionWithHigherFee", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-only",
        walletMock,
      ).lineaRollupClient;

      await expect(lineaRollupClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH)).rejects.toThrow(
        new BaseError("'retryTransactionWithHigherFee' function not callable using readOnly mode."),
      );
    });

    it("should throw an error when priceBumpPercent is not an integer", async () => {
      await expect(lineaRollupClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH, 1.1)).rejects.toThrow(
        new BaseError("'priceBumpPercent' must be an integer"),
      );
    });

    it("should throw an error when getTransaction return null", async () => {
      jest.spyOn(providerMock, "getTransaction").mockResolvedValue(null);

      await expect(lineaRollupClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH)).rejects.toThrow(
        new BaseError(`Transaction with hash ${TEST_TRANSACTION_HASH} not found.`),
      );
    });

    it("should retry the transaction with higher fees", async () => {
      const transactionResponse = generateTransactionResponse();
      const getTransactionSpy = jest.spyOn(providerMock, "getTransaction").mockResolvedValue(transactionResponse);
      const signTransactionSpy = jest.spyOn(walletMock, "signTransaction").mockResolvedValue("");
      const sendTransactionSpy = jest.spyOn(providerMock, "broadcastTransaction");

      await lineaRollupClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH);

      expect(getTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledWith({
        to: transactionResponse.to,
        value: transactionResponse.value,
        data: transactionResponse.data,
        nonce: transactionResponse.nonce,
        gasLimit: transactionResponse.gasLimit,
        chainId: transactionResponse.chainId,
        type: 2,
        maxPriorityFeePerGas: 55000000n,
        maxFeePerGas: 110000000n,
      });
      expect(sendTransactionSpy).toHaveBeenCalledTimes(1);
    });

    it("should retry the transaction with higher fees and capped by the predefined maxFeePerGas", async () => {
      const transactionResponse = generateTransactionResponse();
      const getTransactionSpy = jest.spyOn(providerMock, "getTransaction").mockResolvedValue(transactionResponse);
      const signTransactionSpy = jest.spyOn(walletMock, "signTransaction").mockResolvedValue("");
      const sendTransactionSpy = jest.spyOn(providerMock, "broadcastTransaction");
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-write",
        walletMock,
        {
          maxFeePerGasCap: 500000000n,
        },
      ).lineaRollupClient;

      await lineaRollupClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH, 1000);

      expect(getTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledWith({
        to: transactionResponse.to,
        value: transactionResponse.value,
        data: transactionResponse.data,
        nonce: transactionResponse.nonce,
        gasLimit: transactionResponse.gasLimit,
        chainId: transactionResponse.chainId,
        type: 2,
        maxPriorityFeePerGas: 500000000n,
        maxFeePerGas: 500000000n,
      });
      expect(sendTransactionSpy).toHaveBeenCalledTimes(1);
    });

    it("should retry the transaction with the predefined maxFeePerGas if enforceMaxGasFee is true", async () => {
      const transactionResponse = generateTransactionResponse({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const getTransactionSpy = jest.spyOn(providerMock, "getTransaction").mockResolvedValue(transactionResponse);
      const signTransactionSpy = jest.spyOn(walletMock, "signTransaction").mockResolvedValue("");
      const sendTransactionSpy = jest.spyOn(providerMock, "broadcastTransaction");
      lineaRollupClient = generateLineaRollupClient(
        providerMock,
        l2ProviderMock,
        TEST_CONTRACT_ADDRESS_1,
        TEST_CONTRACT_ADDRESS_2,
        "read-write",
        walletMock,
        {
          maxFeePerGasCap: 500000000n,
          enforceMaxGasFee: true,
        },
      ).lineaRollupClient;

      await lineaRollupClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH, 1000);

      expect(getTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledWith({
        to: transactionResponse.to,
        value: transactionResponse.value,
        data: transactionResponse.data,
        nonce: transactionResponse.nonce,
        gasLimit: transactionResponse.gasLimit,
        chainId: transactionResponse.chainId,
        type: 2,
        maxPriorityFeePerGas: 500000000n,
        maxFeePerGas: 500000000n,
      });
      expect(sendTransactionSpy).toHaveBeenCalledTimes(1);
    });
  });

  describe("isRateLimitExceeded", () => {
    it("should return true if exceeded rate limit", async () => {
      jest.spyOn(lineaRollupMock, "limitInWei").mockResolvedValue(2000000000n);
      jest.spyOn(lineaRollupMock, "currentPeriodAmountInWei").mockResolvedValue(1000000000n);

      const isRateLimitExceeded = await lineaRollupClient.isRateLimitExceeded(1000000000n, 1000000000n);

      expect(isRateLimitExceeded).toBeTruthy();
    });

    it("should return false if not exceeded rate limit", async () => {
      jest.spyOn(lineaRollupMock, "limitInWei").mockResolvedValue(2000000000n);
      jest.spyOn(lineaRollupMock, "currentPeriodAmountInWei").mockResolvedValue(1000000000n);

      const isRateLimitExceeded = await lineaRollupClient.isRateLimitExceeded(100000000n, 100000000n);

      expect(isRateLimitExceeded).toBeFalsy();
    });
  });

  describe("isRateLimitExceededError", () => {
    it("should return false when something went wrong (http error etc)", async () => {
      jest.spyOn(providerMock, "getTransaction").mockRejectedValueOnce({});
      expect(
        await lineaRollupClient.isRateLimitExceededError(
          "0x825a7f1aa4453735597ddf7e9062413c906a7ad49bf17ff32c2cf42f41d438d9",
        ),
      ).toStrictEqual(false);
    });

    it("should return false when transaction revert reason is not RateLimitExceeded", async () => {
      jest.spyOn(providerMock, "getTransaction").mockResolvedValueOnce(generateTransactionResponse());
      jest.spyOn(providerMock, "call").mockResolvedValueOnce("0xa74c1c6d");

      expect(
        await lineaRollupClient.isRateLimitExceededError(
          "0x825a7f1aa4453735597ddf7e9062413c906a7ad49bf17ff32c2cf42f41d438d9",
        ),
      ).toStrictEqual(false);
    });

    it("should return true when transaction revert reason is RateLimitExceeded", async () => {
      mockProperty(lineaRollupMock, "interface", {
        ...lineaRollupMock.interface,
        parseError: jest.fn().mockReturnValueOnce({ name: "RateLimitExceeded" }),
      } as any);
      jest.spyOn(providerMock, "getTransaction").mockResolvedValueOnce(generateTransactionResponse());
      jest.spyOn(providerMock, "call").mockResolvedValueOnce("0xa74c1c5f");

      expect(
        await lineaRollupClient.isRateLimitExceededError(
          "0x825a7f1aa4453735597ddf7e9062413c906a7ad49bf17ff32c2cf42f41d438d9",
        ),
      ).toStrictEqual(true);
    });
  });
});
