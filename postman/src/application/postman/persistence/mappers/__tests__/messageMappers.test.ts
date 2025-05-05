import { describe, it, expect } from "@jest/globals";
import { Direction } from "@consensys/linea-sdk";
import { generateMessage, generateMessageEntity } from "../../../../../utils/testing/helpers";
import { mapMessageEntityToMessage, mapMessageToMessageEntity } from "../messageMappers";
import {
  TEST_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
} from "../../../../../utils/testing/constants";
import { MessageStatus } from "../../../../../core/enums";
import { Message } from "../../../../../core/entities/Message";

describe("Message Mappers", () => {
  describe("mapMessageToMessageEntity", () => {
    it("should map a message to a message entity", () => {
      const message = generateMessage();
      expect(mapMessageToMessageEntity(message)).toStrictEqual({
        calldata: "0x",
        claimGasEstimationThreshold: undefined,
        claimLastRetriedAt: undefined,
        claimNumberOfRetry: 0,
        claimTxCreationDate: undefined,
        claimTxGasLimit: undefined,
        claimTxHash: undefined,
        claimTxMaxFeePerGas: undefined,
        claimTxMaxPriorityFeePerGas: undefined,
        claimTxNonce: undefined,
        compressedTransactionSize: undefined,
        isForSponsorship: false,
        contractAddress: TEST_CONTRACT_ADDRESS_2,
        createdAt: new Date("2023-08-04"),
        destination: TEST_CONTRACT_ADDRESS_1,
        direction: Direction.L1_TO_L2,
        fee: "10",
        id: 1,
        messageContractAddress: TEST_CONTRACT_ADDRESS_2,
        messageHash: TEST_MESSAGE_HASH,
        messageNonce: 1,
        messageSender: TEST_ADDRESS_1,
        sentBlockNumber: 100_000,
        status: MessageStatus.SENT,
        updatedAt: new Date("2023-08-04"),
        value: "2",
      });
    });
  });

  describe("mapMessageToMessageEntity", () => {
    it("should map a message entity to a message", () => {
      const messageEntity = generateMessageEntity();
      expect(mapMessageEntityToMessage(messageEntity)).toStrictEqual(
        new Message({
          calldata: "0x",
          claimGasEstimationThreshold: undefined,
          claimLastRetriedAt: undefined,
          claimNumberOfRetry: 0,
          claimTxCreationDate: undefined,
          claimTxGasLimit: undefined,
          claimTxHash: undefined,
          claimTxMaxFeePerGas: undefined,
          claimTxMaxPriorityFeePerGas: undefined,
          claimTxNonce: undefined,
          contractAddress: TEST_CONTRACT_ADDRESS_2,
          isForSponsorship: false,
          createdAt: new Date("2023-08-04"),
          destination: TEST_CONTRACT_ADDRESS_1,
          direction: Direction.L1_TO_L2,
          fee: 10n,
          id: 1,
          messageHash: TEST_MESSAGE_HASH,
          messageNonce: 1n,
          messageSender: TEST_ADDRESS_1,
          sentBlockNumber: 100_000,
          status: MessageStatus.SENT,
          updatedAt: new Date("2023-08-04"),
          value: 2n,
        }),
      );
    });
  });
});
