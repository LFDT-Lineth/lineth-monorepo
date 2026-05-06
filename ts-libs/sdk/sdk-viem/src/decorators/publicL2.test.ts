import { ExtendedMessage, OnChainMessageStatus } from "@consensys/linea-sdk-core";
import { Client, Transport, Chain, Account, Hex, TransactionReceipt } from "viem";

import { publicActionsL2 } from "./publicL2";
import { TEST_CONTRACT_ADDRESS_2 } from "../../tests/constants";
import { getBlockExtraData } from "../actions/getBlockExtraData";
import { getL1ToL2MessageStatus } from "../actions/getL1ToL2MessageStatus";
import { getMessageByMessageHash } from "../actions/getMessageByMessageHash";
import { getMessagesByTransactionHash } from "../actions/getMessagesByTransactionHash";
import { getTransactionReceiptByMessageHash } from "../actions/getTransactionReceiptByMessageHash";

jest.mock("../actions/getBlockExtraData", () => ({ getBlockExtraData: jest.fn() }));
jest.mock("../actions/getL1ToL2MessageStatus", () => ({ getL1ToL2MessageStatus: jest.fn() }));
jest.mock("../actions/getMessageByMessageHash", () => ({ getMessageByMessageHash: jest.fn() }));
jest.mock("../actions/getMessagesByTransactionHash", () => ({ getMessagesByTransactionHash: jest.fn() }));
jest.mock("../actions/getTransactionReceiptByMessageHash", () => ({ getTransactionReceiptByMessageHash: jest.fn() }));

type MockClient = Client<Transport, Chain, Account>;

describe("publicActionsL2", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({ chain: chainId ? { id: chainId } : undefined }) as unknown as MockClient;

  const client = mockClient(1);

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("with parameters", () => {
    const actions = publicActionsL2({
      l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
    })<Transport, Chain, Account>(client);

    it("delegates getL1ToL2MessageStatus with custom contract addresses to the action", async () => {
      const params: Parameters<typeof actions.getL1ToL2MessageStatus>[0] = {
        messageHash: "0xabc" as Hex,
      };

      (getL1ToL2MessageStatus as jest.Mock<ReturnType<typeof getL1ToL2MessageStatus>>).mockResolvedValue(
        OnChainMessageStatus.CLAIMED,
      );
      const result = await actions.getL1ToL2MessageStatus(params);
      expect(getL1ToL2MessageStatus).toHaveBeenCalledWith(client, {
        ...params,
        l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      });
      expect(result).toBe(OnChainMessageStatus.CLAIMED);
    });

    it("delegates getMessageByMessageHash to the action", async () => {
      const mockMessage = {
        from: "0x0000000000000000000000000000000000000001",
        to: "0x0000000000000000000000000000000000000001",
        fee: 1n,
        value: 2n,
        nonce: 3n,
        calldata: "0x",
        messageHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        transactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        blockNumber: 42n,
      } as ExtendedMessage;
      const params: Parameters<typeof actions.getMessageByMessageHash>[0] = {
        messageHash: "0xabc" as Hex,
      };
      (getMessageByMessageHash as jest.Mock<ReturnType<typeof getMessageByMessageHash>>).mockResolvedValue(mockMessage);
      const result = await actions.getMessageByMessageHash(params);
      expect(getMessageByMessageHash).toHaveBeenCalledWith(client, {
        ...params,
        messageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      });
      expect(result).toBe(mockMessage);
    });

    it("delegates getMessagesByTransactionHash to the action", async () => {
      const mockMessage = {
        from: "0x0000000000000000000000000000000000000001",
        to: "0x0000000000000000000000000000000000000001",
        fee: 1n,
        value: 2n,
        nonce: 3n,
        calldata: "0x",
        messageHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        transactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        blockNumber: 42n,
      } as ExtendedMessage;
      const params: Parameters<typeof actions.getMessagesByTransactionHash>[0] = {
        transactionHash: "0xabc" as Hex,
      };
      (getMessagesByTransactionHash as jest.Mock<ReturnType<typeof getMessagesByTransactionHash>>).mockResolvedValue([
        mockMessage,
      ]);
      const result = await actions.getMessagesByTransactionHash(params);
      expect(getMessagesByTransactionHash).toHaveBeenCalledWith(client, {
        ...params,
        messageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      });
      expect(result).toEqual([mockMessage]);
    });

    it("delegates getTransactionReceiptByMessageHash to the action", async () => {
      const transactionReceipt = {
        status: "success",
        blockNumber: 42n,
        transactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        blockHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        contractAddress: "0x0000000000000000000000000000000000000001",
        cumulativeGasUsed: 100000n,
        from: "0x0000000000000000000000000000000000000001",
        effectiveGasPrice: 10000n,
        gasUsed: 10000n,
        logs: [],
        logsBloom: "0x",
        to: "0x0000000000000000000000000000000000000001",
        transactionIndex: 1,
        type: "eip1559",
      } as TransactionReceipt;
      const params: Parameters<typeof actions.getTransactionReceiptByMessageHash>[0] = {
        messageHash: "0xabc" as Hex,
      };
      (
        getTransactionReceiptByMessageHash as jest.Mock<ReturnType<typeof getTransactionReceiptByMessageHash>>
      ).mockResolvedValue(transactionReceipt);
      const result = await actions.getTransactionReceiptByMessageHash(params);
      expect(getTransactionReceiptByMessageHash).toHaveBeenCalledWith(client, {
        ...params,
        messageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      });
      expect(result).toBe(transactionReceipt);
    });
  });

  describe("without parameters", () => {
    const actions = publicActionsL2()<Transport, Chain, Account>(client);

    it("delegates getBlockExtraData to the action", async () => {
      const blockExtraData = { version: 1, fixedCost: 2, variableCost: 3, ethGasPrice: 4 };
      const params: Parameters<typeof actions.getBlockExtraData>[0] = { blockTag: "latest" };
      (getBlockExtraData as jest.Mock<ReturnType<typeof getBlockExtraData>>).mockResolvedValue(blockExtraData);
      const result = await actions.getBlockExtraData(params);
      expect(getBlockExtraData).toHaveBeenCalledWith(client, params);
      expect(result).toBe(blockExtraData);
    });

    it("delegates getL1ToL2MessageStatus to the action", async () => {
      const params: Parameters<typeof actions.getL1ToL2MessageStatus>[0] = { messageHash: "0xabc" as Hex };
      (getL1ToL2MessageStatus as jest.Mock<ReturnType<typeof getL1ToL2MessageStatus>>).mockResolvedValue(
        OnChainMessageStatus.CLAIMED,
      );
      const result = await actions.getL1ToL2MessageStatus(params);
      expect(getL1ToL2MessageStatus).toHaveBeenCalledWith(client, params);
      expect(result).toBe(OnChainMessageStatus.CLAIMED);
    });

    it("delegates getMessageByMessageHash to the action", async () => {
      const mockMessage = {
        from: "0x0000000000000000000000000000000000000001",
        to: "0x0000000000000000000000000000000000000001",
        fee: 1n,
        value: 2n,
        nonce: 3n,
        calldata: "0x",
        messageHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        transactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        blockNumber: 42n,
      } as ExtendedMessage;
      const params: Parameters<typeof actions.getMessageByMessageHash>[0] = { messageHash: "0xabc" as Hex };
      (getMessageByMessageHash as jest.Mock<ReturnType<typeof getMessageByMessageHash>>).mockResolvedValue(mockMessage);
      const result = await actions.getMessageByMessageHash(params);
      expect(getMessageByMessageHash).toHaveBeenCalledWith(client, params);
      expect(result).toBe(mockMessage);
    });

    it("delegates getMessagesByTransactionHash to the action", async () => {
      const mockMessage = {
        from: "0x0000000000000000000000000000000000000001",
        to: "0x0000000000000000000000000000000000000001",
        fee: 1n,
        value: 2n,
        nonce: 3n,
        calldata: "0x",
        messageHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        transactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        blockNumber: 42n,
      } as ExtendedMessage;
      const params: Parameters<typeof actions.getMessagesByTransactionHash>[0] = { transactionHash: "0xabc" as Hex };
      (getMessagesByTransactionHash as jest.Mock<ReturnType<typeof getMessagesByTransactionHash>>).mockResolvedValue([
        mockMessage,
      ]);
      const result = await actions.getMessagesByTransactionHash(params);
      expect(getMessagesByTransactionHash).toHaveBeenCalledWith(client, params);
      expect(result).toEqual([mockMessage]);
    });

    it("delegates getTransactionReceiptByMessageHash to the action", async () => {
      const transactionReceipt = {
        status: "success",
        blockNumber: 42n,
        transactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        blockHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        contractAddress: "0x0000000000000000000000000000000000000001",
        cumulativeGasUsed: 100000n,
        from: "0x0000000000000000000000000000000000000001",
        effectiveGasPrice: 10000n,
        gasUsed: 10000n,
        logs: [],
        logsBloom: "0x",
        to: "0x0000000000000000000000000000000000000001",
        transactionIndex: 1,
        type: "eip1559",
      } as TransactionReceipt;
      const params: Parameters<typeof actions.getTransactionReceiptByMessageHash>[0] = { messageHash: "0xabc" as Hex };
      (
        getTransactionReceiptByMessageHash as jest.Mock<ReturnType<typeof getTransactionReceiptByMessageHash>>
      ).mockResolvedValue(transactionReceipt);
      const result = await actions.getTransactionReceiptByMessageHash(params);
      expect(getTransactionReceiptByMessageHash).toHaveBeenCalledWith(client, params);
      expect(result).toBe(transactionReceipt);
    });
  });
});
