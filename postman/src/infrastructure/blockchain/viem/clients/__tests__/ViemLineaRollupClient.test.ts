import { claimOnL1, getL2ToL1MessageStatus, getMessageProof } from "@consensys/linea-sdk-viem";
import { describe, it, expect, beforeEach, afterEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { decodeErrorResult, type PublicClient, type WalletClient } from "viem";
import { estimateContractGas, readContract } from "viem/actions";

import { IEthereumGasProvider } from "../../../../../core/clients/blockchain/IGasProvider";
import { OnChainMessageStatus } from "../../../../../core/enums";
import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_CLAIM_VIA_ADDRESS,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MERKLE_ROOT,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
  testMessageSentEvent,
} from "../../../../../utils/testing/constants";
import { ViemLineaRollupClient } from "../ViemLineaRollupClient";

jest.mock("@consensys/linea-sdk-viem", () => ({
  claimOnL1: jest.fn(),
  getL2ToL1MessageStatus: jest.fn(),
  getMessageProof: jest.fn(),
  getMessagesByTransactionHash: jest.fn(),
  getTransactionReceiptByMessageHash: jest.fn(),
}));
jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return { ...actual, decodeErrorResult: jest.fn() };
});
jest.mock("viem/actions", () => ({
  estimateContractGas: jest.fn(),
  getContractEvents: jest.fn(),
  readContract: jest.fn(),
}));

describe("ViemLineaRollupClient", () => {
  let publicClient: ReturnType<typeof mock<PublicClient>>;
  let walletClient: ReturnType<typeof mock<WalletClient>>;
  let l2PublicClient: ReturnType<typeof mock<PublicClient>>;
  let gasProvider: ReturnType<typeof mock<IEthereumGasProvider>>;
  let client: ViemLineaRollupClient;

  beforeEach(() => {
    publicClient = mock<PublicClient>();
    walletClient = mock<WalletClient>();
    l2PublicClient = mock<PublicClient>();
    gasProvider = mock<IEthereumGasProvider>();

    client = new ViemLineaRollupClient(
      publicClient,
      walletClient,
      TEST_CONTRACT_ADDRESS_1,
      l2PublicClient,
      TEST_CONTRACT_ADDRESS_2,
      gasProvider,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("getMessageStatus", () => {
    it("returns CLAIMED", async () => {
      (getL2ToL1MessageStatus as jest.Mock).mockResolvedValue(OnChainMessageStatus.CLAIMED);
      const result = await client.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });
      expect(result).toBe(OnChainMessageStatus.CLAIMED);
    });

    it("returns CLAIMABLE", async () => {
      (getL2ToL1MessageStatus as jest.Mock).mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      const result = await client.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });
      expect(result).toBe(OnChainMessageStatus.CLAIMABLE);
    });

    it("returns UNKNOWN", async () => {
      (getL2ToL1MessageStatus as jest.Mock).mockResolvedValue(OnChainMessageStatus.UNKNOWN);
      const result = await client.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });
      expect(result).toBe(OnChainMessageStatus.UNKNOWN);
    });

    it("narrows block range when messageBlockNumber is provided", async () => {
      (getL2ToL1MessageStatus as jest.Mock).mockResolvedValue(OnChainMessageStatus.CLAIMABLE);
      await client.getMessageStatus({ messageHash: TEST_MESSAGE_HASH, messageBlockNumber: 51 });
      expect(getL2ToL1MessageStatus).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({ l2LogsBlockRange: { fromBlock: 51n, toBlock: 51n } }),
      );
    });
  });

  describe("isRateLimitExceeded", () => {
    it("returns false when total is within limit", async () => {
      (readContract as jest.Mock)
        .mockResolvedValueOnce(1000n) // limitInWei
        .mockResolvedValueOnce(100n); // currentPeriodAmountInWei

      const result = await client.isRateLimitExceeded(50n, 50n);
      // 100 + 50 + 50 = 200; 1000 * 0.95 = 950; 200 <= 950
      expect(result).toBe(false);
    });

    it("returns true when total exceeds limit * margin", async () => {
      (readContract as jest.Mock)
        .mockResolvedValueOnce(1000n) // limitInWei
        .mockResolvedValueOnce(950n); // currentPeriodAmountInWei

      const result = await client.isRateLimitExceeded(100n, 0n);
      // 950 + 100 + 0 = 1050; 1000 * 0.95 = 950; 1050 > 950
      expect(result).toBe(true);
    });
  });

  describe("isRateLimitExceededError", () => {
    it("returns false for string errors", async () => {
      publicClient.getTransaction.mockResolvedValue(
        null as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      const result = await client.isRateLimitExceededError(TEST_TRANSACTION_HASH);
      expect(result).toBe(false);
    });

    it("returns true when parsed error name is RateLimitExceeded", async () => {
      const mockTx = {
        to: TEST_ADDRESS_2,
        from: TEST_ADDRESS_1,
        nonce: 1,
        gas: 21000n,
        input: "0x" as `0x${string}`,
        value: 0n,
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 100n,
      };
      publicClient.getTransaction.mockResolvedValue(
        mockTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockResolvedValue({ data: "0xdeadbeef" } as never);
      (decodeErrorResult as jest.Mock).mockReturnValue({ errorName: "RateLimitExceeded", args: [] });

      const result = await client.isRateLimitExceededError(TEST_TRANSACTION_HASH);
      expect(result).toBe(true);
    });
  });

  describe("getMessageProof", () => {
    it("delegates to SDK getMessageProof with correct params", async () => {
      const mockProof = {
        proof: [TEST_MERKLE_ROOT as `0x${string}`],
        root: TEST_MERKLE_ROOT as `0x${string}`,
        leafIndex: 0,
      };
      (getMessageProof as jest.Mock).mockResolvedValue(mockProof);

      const result = await client.getMessageProof(TEST_MESSAGE_HASH);
      expect(result).toEqual(mockProof);
      expect(getMessageProof).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          l2Client: l2PublicClient,
          messageHash: TEST_MESSAGE_HASH,
          lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
          l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
        }),
      );
    });

    it("narrows block range when messageBlockNumber is provided", async () => {
      const mockProof = {
        proof: [TEST_MERKLE_ROOT as `0x${string}`],
        root: TEST_MERKLE_ROOT as `0x${string}`,
        leafIndex: 0,
      };
      (getMessageProof as jest.Mock).mockResolvedValue(mockProof);

      await client.getMessageProof(TEST_MESSAGE_HASH, 51);
      expect(getMessageProof).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          l2LogsBlockRange: { fromBlock: 51n, toBlock: 51n },
        }),
      );
    });
  });

  describe("claim", () => {
    it("uses claimOnL1 from sdk-viem and returns TransactionSubmission", async () => {
      (getMessageProof as jest.Mock).mockResolvedValue({
        proof: [TEST_MERKLE_ROOT as `0x${string}`],
        root: TEST_MERKLE_ROOT as `0x${string}`,
        leafIndex: 0,
      });
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      (claimOnL1 as jest.Mock).mockResolvedValue("0xclaimhash" as `0x${string}`);

      const result = await client.claim(testMessageSentEvent, {
        overrides: { nonce: 2, gasLimit: 150000n, maxFeePerGas: 800n, maxPriorityFeePerGas: 80n },
      });

      expect(result.hash).toBe("0xclaimhash");
      expect(result.nonce).toBe(2);
      expect(result.gasLimit).toBe(150000n);
      expect(claimOnL1).toHaveBeenCalledWith(
        walletClient,
        expect.objectContaining({
          messageNonce: testMessageSentEvent.messageNonce,
          lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
        }),
      );
    });

    it("falls back to gasProvider fees and zero nonce/gasLimit when no overrides are provided", async () => {
      (getMessageProof as jest.Mock).mockResolvedValue({
        proof: [TEST_MERKLE_ROOT as `0x${string}`],
        root: TEST_MERKLE_ROOT as `0x${string}`,
        leafIndex: 0,
      });
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      (claimOnL1 as jest.Mock).mockResolvedValue("0xclaimhash" as `0x${string}`);

      const result = await client.claim(testMessageSentEvent);

      expect(result.hash).toBe("0xclaimhash");
      expect(result.nonce).toBe(0);
      expect(result.gasLimit).toBe(0n);
      expect(result.maxFeePerGas).toBe(1000n);
      expect(result.maxPriorityFeePerGas).toBe(100n);
      expect(claimOnL1).toHaveBeenCalledWith(
        walletClient,
        expect.objectContaining({
          lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
          maxFeePerGas: 1000n,
          maxPriorityFeePerGas: 100n,
        }),
      );
    });

    it("uses messageBlockNumber from message when present", async () => {
      (getMessageProof as jest.Mock).mockResolvedValue({
        proof: [TEST_MERKLE_ROOT as `0x${string}`],
        root: TEST_MERKLE_ROOT as `0x${string}`,
        leafIndex: 0,
      });
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      (claimOnL1 as jest.Mock).mockResolvedValue("0xclaimhash" as `0x${string}`);

      const messageWithBlockNumber = { ...testMessageSentEvent, messageBlockNumber: 51 };
      await client.claim(messageWithBlockNumber);

      expect(getMessageProof).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          l2LogsBlockRange: { fromBlock: 51n, toBlock: 51n },
        }),
      );
    });

    it("uses claimViaAddress when provided", async () => {
      (getMessageProof as jest.Mock).mockResolvedValue({
        proof: [TEST_MERKLE_ROOT as `0x${string}`],
        root: TEST_MERKLE_ROOT as `0x${string}`,
        leafIndex: 0,
      });
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      (claimOnL1 as jest.Mock).mockResolvedValue("0xclaimhash" as `0x${string}`);

      await client.claim(testMessageSentEvent, { claimViaAddress: TEST_CLAIM_VIA_ADDRESS });

      expect(claimOnL1).toHaveBeenCalledWith(
        walletClient,
        expect.objectContaining({
          lineaRollupAddress: TEST_CLAIM_VIA_ADDRESS,
        }),
      );
    });
  });

  describe("estimateClaimGas", () => {
    it("calls getMessageProof, gasProvider, walletClient.getAddresses, and estimateContractGas", async () => {
      const mockProof = {
        proof: [TEST_MERKLE_ROOT],
        root: TEST_MERKLE_ROOT,
        leafIndex: 0,
      };
      (getMessageProof as jest.Mock).mockResolvedValue(mockProof);
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      walletClient.getAddresses.mockResolvedValue([TEST_ADDRESS_1] as never);
      (estimateContractGas as jest.Mock).mockResolvedValue(50000n);

      const result = await client.estimateClaimGas(testMessageSentEvent);

      expect(getMessageProof).toHaveBeenCalled();
      expect(gasProvider.getGasFees).toHaveBeenCalled();
      expect(walletClient.getAddresses).toHaveBeenCalled();
      expect(estimateContractGas).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          address: TEST_CONTRACT_ADDRESS_1,
          functionName: "claimMessageWithProof",
          account: TEST_ADDRESS_1,
        }),
      );
      expect(result).toBe(50000n);
    });

    it("passes messageBlockNumber to getMessageProof when present in message", async () => {
      (getMessageProof as jest.Mock).mockResolvedValue({
        proof: [TEST_MERKLE_ROOT],
        root: TEST_MERKLE_ROOT,
        leafIndex: 0,
      });
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      walletClient.getAddresses.mockResolvedValue([TEST_ADDRESS_1] as never);
      (estimateContractGas as jest.Mock).mockResolvedValue(50000n);

      const messageWithBlockNumber = { ...testMessageSentEvent, messageBlockNumber: 51 };
      await client.estimateClaimGas(messageWithBlockNumber);

      expect(getMessageProof).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          l2LogsBlockRange: { fromBlock: 51n, toBlock: 51n },
        }),
      );
    });

    it("uses claimViaAddress when provided in opts", async () => {
      (getMessageProof as jest.Mock).mockResolvedValue({
        proof: [TEST_MERKLE_ROOT],
        root: TEST_MERKLE_ROOT,
        leafIndex: 0,
      });
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      walletClient.getAddresses.mockResolvedValue([TEST_ADDRESS_1] as never);
      (estimateContractGas as jest.Mock).mockResolvedValue(50000n);

      await client.estimateClaimGas(testMessageSentEvent, { claimViaAddress: TEST_CLAIM_VIA_ADDRESS });

      expect(estimateContractGas).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          address: TEST_CLAIM_VIA_ADDRESS,
        }),
      );
    });

    it("uses override fees when provided instead of gasProvider fees", async () => {
      (getMessageProof as jest.Mock).mockResolvedValue({
        proof: [TEST_MERKLE_ROOT],
        root: TEST_MERKLE_ROOT,
        leafIndex: 0,
      });
      gasProvider.getGasFees.mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });
      walletClient.getAddresses.mockResolvedValue([TEST_ADDRESS_1] as never);
      (estimateContractGas as jest.Mock).mockResolvedValue(50000n);

      await client.estimateClaimGas(testMessageSentEvent, {
        overrides: { maxFeePerGas: 500n, maxPriorityFeePerGas: 50n },
      });

      expect(estimateContractGas).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          maxFeePerGas: 500n,
          maxPriorityFeePerGas: 50n,
        }),
      );
    });
  });

  describe("parseTransactionError", () => {
    const mockTx = {
      to: TEST_ADDRESS_2,
      from: TEST_ADDRESS_1,
      nonce: 1,
      gas: 21000n,
      input: "0x" as `0x${string}`,
      value: 0n,
      maxFeePerGas: 1000n,
      maxPriorityFeePerGas: 100n,
    };

    it("returns decoded error when call returns revert data", async () => {
      publicClient.getTransaction.mockResolvedValue(
        mockTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockResolvedValue({ data: "0xdeadbeef" } as never);
      (decodeErrorResult as jest.Mock).mockReturnValue({ errorName: "SomeError", args: [42n] });

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);
      expect(result).toEqual({ name: "SomeError", args: [42n] });
    });

    it("extracts error data from call exception", async () => {
      publicClient.getTransaction.mockResolvedValue(
        mockTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockRejectedValue({ data: "0xcafebabe" as `0x${string}` });
      (decodeErrorResult as jest.Mock).mockReturnValue({ errorName: "CallError", args: [] });

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);
      expect(result).toEqual({ name: "CallError", args: [] });
    });

    it("returns '0x' when call throws without revert data", async () => {
      publicClient.getTransaction.mockResolvedValue(
        mockTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockRejectedValue(new Error("execution reverted"));

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);
      expect(result).toBe("0x");
    });

    it("returns '0x' when call returns no data", async () => {
      publicClient.getTransaction.mockResolvedValue(
        mockTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockResolvedValue({ data: undefined } as never);

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);
      expect(result).toBe("0x");
    });

    it("returns raw encoded data when decode throws", async () => {
      publicClient.getTransaction.mockResolvedValue(
        mockTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockResolvedValue({ data: "0xdeadbeef" } as never);
      (decodeErrorResult as jest.Mock).mockImplementation(() => {
        throw new Error("decode failed");
      });

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);
      expect(result).toBe("0xdeadbeef");
    });

    it("passes undefined for null tx.to, tx.maxFeePerGas, and tx.maxPriorityFeePerGas", async () => {
      const nullFieldsTx = {
        to: null,
        from: TEST_ADDRESS_1,
        nonce: 1,
        gas: 21000n,
        input: "0x" as `0x${string}`,
        value: 0n,
        maxFeePerGas: null,
        maxPriorityFeePerGas: null,
      };
      publicClient.getTransaction.mockResolvedValue(
        nullFieldsTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockResolvedValue({ data: "0xdeadbeef" } as never);
      (decodeErrorResult as jest.Mock).mockReturnValue({ errorName: "SomeError", args: [1n] });

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);

      expect(publicClient.call).toHaveBeenCalledWith(
        expect.objectContaining({
          to: undefined,
          maxFeePerGas: undefined,
          maxPriorityFeePerGas: undefined,
        }),
      );
      expect(result).toEqual({ name: "SomeError", args: [1n] });
    });

    it("returns empty args array when decoded.args is undefined", async () => {
      publicClient.getTransaction.mockResolvedValue(
        mockTx as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      publicClient.call.mockResolvedValue({ data: "0xdeadbeef" } as never);
      (decodeErrorResult as jest.Mock).mockReturnValue({ errorName: "SomeError", args: undefined });

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);
      expect(result).toEqual({ name: "SomeError", args: [] });
    });

    it("returns '0x' when getTransaction returns null", async () => {
      publicClient.getTransaction.mockResolvedValue(
        null as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );

      const result = await client.parseTransactionError(TEST_TRANSACTION_HASH);
      expect(result).toBe("0x");
    });
  });
});
