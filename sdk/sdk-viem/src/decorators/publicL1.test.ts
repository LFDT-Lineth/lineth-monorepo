import { ExtendedMessage, MessageProof, OnChainMessageStatus } from "@consensys/linea-sdk-core";
import { Client, Transport, Chain, Account, Hex, TransactionReceipt } from "viem";

import { publicActionsL1 } from "./publicL1";
import { TEST_CONTRACT_ADDRESS_1, TEST_CONTRACT_ADDRESS_2 } from "../../tests/constants";
import { getL2ToL1MessageStatus } from "../actions/getL2ToL1MessageStatus";
import { getMessageByMessageHash } from "../actions/getMessageByMessageHash";
import { getMessageProof } from "../actions/getMessageProof";
import { getMessagesByTransactionHash } from "../actions/getMessagesByTransactionHash";
import { getTransactionReceiptByMessageHash } from "../actions/getTransactionReceiptByMessageHash";

jest.mock("../actions/getMessageProof", () => ({ getMessageProof: jest.fn() }));
jest.mock("../actions/getL2ToL1MessageStatus", () => ({ getL2ToL1MessageStatus: jest.fn() }));
jest.mock("../actions/getMessageByMessageHash", () => ({ getMessageByMessageHash: jest.fn() }));
jest.mock("../actions/getMessagesByTransactionHash", () => ({ getMessagesByTransactionHash: jest.fn() }));
jest.mock("../actions/getTransactionReceiptByMessageHash", () => ({ getTransactionReceiptByMessageHash: jest.fn() }));

type MockClient = Client<Transport, Chain, Account>;

describe("publicActionsL1", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({ chain: chainId ? { id: chainId } : undefined }) as unknown as MockClient;

  const client = mockClient(1);

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("with parameters", () => {
    const actions = publicActionsL1({
      lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
      l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
    })<Chain, Account>(client);

    it("delegates getMessageProof to the action", async () => {
      const messageProof: MessageProof = {
        proof: [],
        root: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        leafIndex: 0,
      };
      const params: Parameters<typeof actions.getMessageProof>[0] = {
        l2Client: client,
        messageHash: "0xabc" as Hex,
      };
      (getMessageProof as jest.Mock<ReturnType<typeof getMessageProof>>).mockResolvedValue(messageProof);

      const result = await actions.getMessageProof(params);

      expect(getMessageProof).toHaveBeenCalledWith(client, {
        ...params,
        lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
        l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      });
      expect(result).toBe(messageProof);
    });

    it("delegates getL2ToL1MessageStatus to the action", async () => {
      const params: Parameters<typeof actions.getL2ToL1MessageStatus>[0] = {
        l2Client: client,
        messageHash: "0xabc" as Hex,
      };
      (getL2ToL1MessageStatus as jest.Mock<ReturnType<typeof getL2ToL1MessageStatus>>).mockResolvedValue(
        OnChainMessageStatus.CLAIMED,
      );

      const result = await actions.getL2ToL1MessageStatus(params);

      expect(getL2ToL1MessageStatus).toHaveBeenCalledWith(client, {
        ...params,
        lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
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
        messageServiceAddress: TEST_CONTRACT_ADDRESS_1,
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
        messageServiceAddress: TEST_CONTRACT_ADDRESS_1,
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
        messageServiceAddress: TEST_CONTRACT_ADDRESS_1,
      });
      expect(result).toBe(transactionReceipt);
    });
  });

  describe("without parameters", () => {
    const actions = publicActionsL1()<Chain, Account>(client);

    it("delegates getMessageProof to the action", async () => {
      const messageProof: MessageProof = {
        proof: [],
        root: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        leafIndex: 0,
      };
      const params: Parameters<typeof actions.getMessageProof>[0] = {
        l2Client: client,
        messageHash: "0xabc" as Hex,
      };
      (getMessageProof as jest.Mock<ReturnType<typeof getMessageProof>>).mockResolvedValue(messageProof);

      const result = await actions.getMessageProof(params);

      expect(getMessageProof).toHaveBeenCalledWith(client, params);
      expect(result).toBe(messageProof);
    });

    it("delegates getL2ToL1MessageStatus to the action", async () => {
      const params: Parameters<typeof actions.getL2ToL1MessageStatus>[0] = {
        l2Client: client,
        messageHash: "0xabc" as Hex,
      };
      (getL2ToL1MessageStatus as jest.Mock<ReturnType<typeof getL2ToL1MessageStatus>>).mockResolvedValue(
        OnChainMessageStatus.CLAIMED,
      );

      const result = await actions.getL2ToL1MessageStatus(params);

      expect(getL2ToL1MessageStatus).toHaveBeenCalledWith(client, params);
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
