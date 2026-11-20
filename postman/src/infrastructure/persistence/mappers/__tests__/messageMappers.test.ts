import { describe, it, expect } from "@jest/globals";

import { Message } from "../../../../core/entities/Message";
import { Direction, MessageStatus } from "../../../../core/enums";
import {
  TEST_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
} from "../../../../utils/testing/constants";
import { generateMessage, generateMessageEntity } from "../../../../utils/testing/helpers";
import { mapMessageEntityToMessage, mapMessageToMessageEntity } from "../messageMappers";

describe("Message Mappers", () => {
  describe("mapMessageToMessageEntity", () => {
    it("should map a message to a message entity", () => {
      const message = generateMessage();
      expect(mapMessageToMessageEntity(message)).toStrictEqual({
        calldata: "0x",
        claimGasEstimationThreshold: undefined,
        claimLastRetriedAt: undefined,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
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

  describe("mapMessageToMessageEntity with undefined dates", () => {
    it("should use new Date() when createdAt and updatedAt are undefined", () => {
      const now = new Date();
      jest.useFakeTimers({ now });

      const message = generateMessage({ createdAt: undefined, updatedAt: undefined });
      const entity = mapMessageToMessageEntity(message);

      expect(entity.createdAt).toStrictEqual(now);
      expect(entity.updatedAt).toStrictEqual(now);

      jest.useRealTimers();
    });

    it("should preserve explicit createdAt and updatedAt dates", () => {
      const explicit = new Date("2025-01-15");
      const message = generateMessage({ createdAt: explicit, updatedAt: explicit });
      const entity = mapMessageToMessageEntity(message);

      expect(entity.createdAt).toStrictEqual(explicit);
      expect(entity.updatedAt).toStrictEqual(explicit);
    });
  });

  describe("mapMessageEntityToMessage", () => {
    it("should map a message entity to a message", () => {
      const messageEntity = generateMessageEntity();
      expect(mapMessageEntityToMessage(messageEntity)).toStrictEqual(
        new Message({
          calldata: "0x",
          claimGasEstimationThreshold: undefined,
          claimLastRetriedAt: undefined,
          claimNumberOfRetry: 0,
          claimCycleCount: 0,
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

    it("should default claimCycleCount to 0 when entity has undefined claimCycleCount", () => {
      const entity = generateMessageEntity({ claimCycleCount: undefined as unknown as number });
      const message = mapMessageEntityToMessage(entity);
      expect(message.claimCycleCount).toBe(0);
    });

    it("should use explicit claimCycleCount when provided", () => {
      const entity = generateMessageEntity({ claimCycleCount: 5 });
      const message = mapMessageEntityToMessage(entity);
      expect(message.claimCycleCount).toBe(5);
    });
  });

  describe("mapMessageEntityToMessage with bigint columns read as strings", () => {
    it("converts claim tx fee strings from the pg driver to bigint", () => {
      const entity = generateMessageEntity();
      const message = mapMessageEntityToMessage({
        ...entity,
        claimTxMaxFeePerGas: "3819885" as unknown as bigint,
        claimTxMaxPriorityFeePerGas: "3819878" as unknown as bigint,
      });
      expect(message.claimTxMaxFeePerGas).toBe(3819885n);
      expect(message.claimTxMaxPriorityFeePerGas).toBe(3819878n);
      expect(((message.claimTxMaxFeePerGas as bigint) * 110n) / 100n).toBe(4201873n);
    });

    it("keeps missing claim tx fees undefined", () => {
      const entity = generateMessageEntity();
      const message = mapMessageEntityToMessage({
        ...entity,
        claimTxMaxFeePerGas: null as unknown as bigint,
        claimTxMaxPriorityFeePerGas: undefined,
      });
      expect(message.claimTxMaxFeePerGas).toBeUndefined();
      expect(message.claimTxMaxPriorityFeePerGas).toBeUndefined();
    });
  });
});
