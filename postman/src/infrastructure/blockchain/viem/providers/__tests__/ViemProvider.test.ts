import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { TEST_ADDRESS_1, TEST_TRANSACTION_HASH } from "../../../../../utils/testing/constants";
import { TestLogger } from "../../../../../utils/testing/helpers";
import { ViemProvider } from "../ViemProvider";

import type { PublicClient } from "viem";

describe("ViemProvider", () => {
  let publicClient: ReturnType<typeof mock<PublicClient>>;
  let provider: ViemProvider;
  const logger = new TestLogger("TestViemProvider");

  beforeEach(() => {
    publicClient = mock<PublicClient>();
    provider = new ViemProvider(publicClient, logger);
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("getBlockNumber", () => {
    it("returns block number as a number", async () => {
      publicClient.getBlockNumber.mockResolvedValue(12345n);
      await expect(provider.getBlockNumber()).resolves.toBe(12345);
    });
  });

  describe("getTransactionCount", () => {
    it("returns transaction count", async () => {
      publicClient.getTransactionCount.mockResolvedValue(5);
      await expect(provider.getTransactionCount(TEST_ADDRESS_1, "latest")).resolves.toBe(5);
      expect(publicClient.getTransactionCount).toHaveBeenCalledWith({
        address: TEST_ADDRESS_1,
        blockTag: "latest",
      });
    });
  });

  describe("getTransactionReceipt", () => {
    it("returns mapped receipt when found", async () => {
      publicClient.getTransactionReceipt.mockResolvedValue({
        transactionHash: TEST_TRANSACTION_HASH,
        blockNumber: 100n,
        status: "success",
        gasUsed: 21000n,
        effectiveGasPrice: 1000000000n,
        logs: [],
        blockHash: "0xabc",
        contractAddress: null,
        cumulativeGasUsed: 21000n,
        from: TEST_ADDRESS_1,
        logsBloom: "0x",
        to: TEST_ADDRESS_1,
        transactionIndex: 0,
        type: "eip1559",
      } as never);

      const receipt = await provider.getTransactionReceipt(TEST_TRANSACTION_HASH);
      expect(receipt).toMatchObject({
        hash: TEST_TRANSACTION_HASH,
        blockNumber: 100,
        status: "success",
        gasUsed: 21000n,
        gasPrice: 1000000000n,
        logs: [],
      });
    });

    it("returns null and logs warning when receipt not found", async () => {
      const rpcError = new Error("not found");
      publicClient.getTransactionReceipt.mockRejectedValue(rpcError);
      const loggerDebugSpy = jest.spyOn(logger, "debug");

      await expect(provider.getTransactionReceipt(TEST_TRANSACTION_HASH)).resolves.toBeNull();
      expect(loggerDebugSpy).toHaveBeenCalledWith("Failed to fetch transaction receipt.", {
        txHash: TEST_TRANSACTION_HASH,
        error: rpcError,
      });
    });
  });

  describe("getBlock", () => {
    it("returns mapped block for a numeric block number", async () => {
      publicClient.getBlock.mockResolvedValue({
        number: 50n,
        timestamp: 1700000000n,
        hash: "0xblockhash",
        baseFeePerGas: 1000n,
        difficulty: 0n,
        extraData: "0x",
        gasLimit: 30000000n,
        gasUsed: 15000000n,
        logsBloom: "0x",
        miner: TEST_ADDRESS_1,
        mixHash: "0x",
        nonce: "0x0000000000000000",
        parentHash: "0xparent",
        receiptsRoot: "0x",
        sealFields: [],
        sha3Uncles: "0x",
        size: 1000n,
        stateRoot: "0x",
        totalDifficulty: 0n,
        transactions: [],
        transactionsRoot: "0x",
        uncles: [],
        withdrawals: [],
        withdrawalsRoot: "0x",
      } as never);

      const block = await provider.getBlock(50n);
      expect(block).toMatchObject({ number: 50, timestamp: 1700000000, hash: "0xblockhash" });
      expect(publicClient.getBlock).toHaveBeenCalledWith({ blockNumber: 50n });
    });

    it("returns mapped block for a string tag", async () => {
      publicClient.getBlock.mockResolvedValue({
        number: 99n,
        timestamp: 1700000099n,
        hash: "0xlatesthash",
      } as never);

      const block = await provider.getBlock("latest");
      expect(block).toMatchObject({ number: 99, hash: "0xlatesthash" });
      expect(publicClient.getBlock).toHaveBeenCalledWith({ blockTag: "latest" });
    });

    it("returns null and logs warning on error", async () => {
      const rpcError = new Error("block not found");
      publicClient.getBlock.mockRejectedValue(rpcError);
      const loggerDebugSpy = jest.spyOn(logger, "debug");

      await expect(provider.getBlock(9999n)).resolves.toBeNull();
      expect(loggerDebugSpy).toHaveBeenCalledWith("Failed to fetch block.", {
        blockNumber: 9999n,
        error: rpcError,
      });
    });
  });

  describe("estimateGas", () => {
    it("returns estimated gas", async () => {
      publicClient.estimateGas.mockResolvedValue(50000n);
      const result = await provider.estimateGas({ to: TEST_ADDRESS_1 });
      expect(result).toBe(50000n);
    });
  });

  describe("getTransaction", () => {
    it("returns mapped transaction submission", async () => {
      publicClient.getTransaction.mockResolvedValue({
        hash: TEST_TRANSACTION_HASH,
        nonce: 7,
        gas: 21000n,
        maxFeePerGas: 2000000000n,
        maxPriorityFeePerGas: 1000000000n,
        blockHash: "0x",
        blockNumber: 100n,
        from: TEST_ADDRESS_1,
        input: "0x",
        r: "0x",
        s: "0x",
        to: TEST_ADDRESS_1,
        transactionIndex: 0,
        typeHex: "0x2",
        v: 0n,
        value: 0n,
        type: "eip1559",
        chainId: 1,
        accessList: [],
        yParity: 0,
      } as never);

      const tx = await provider.getTransaction(TEST_TRANSACTION_HASH);
      expect(tx).toMatchObject({
        hash: TEST_TRANSACTION_HASH,
        nonce: 7,
        gasLimit: 21000n,
        maxFeePerGas: 2000000000n,
        maxPriorityFeePerGas: 1000000000n,
      });
    });

    it("returns null and logs warning when transaction not found", async () => {
      const rpcError = new Error("not found");
      publicClient.getTransaction.mockRejectedValue(rpcError);
      const loggerDebugSpy = jest.spyOn(logger, "debug");

      await expect(provider.getTransaction(TEST_TRANSACTION_HASH)).resolves.toBeNull();
      expect(loggerDebugSpy).toHaveBeenCalledWith("Failed to fetch transaction.", {
        transactionHash: TEST_TRANSACTION_HASH,
        error: rpcError,
      });
    });
  });

  describe("call", () => {
    it("returns the response data", async () => {
      publicClient.call.mockResolvedValue({ data: "0xdeadbeef" });
      const result = await provider.call({ to: TEST_ADDRESS_1 });
      expect(result).toBe("0xdeadbeef");
    });

    it("returns 0x when data is undefined", async () => {
      publicClient.call.mockResolvedValue({ data: undefined });
      const result = await provider.call({ to: TEST_ADDRESS_1 });
      expect(result).toBe("0x");
    });
  });

  describe("getFees", () => {
    it("returns estimated fees per gas", async () => {
      publicClient.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 3000000000n,
        maxPriorityFeePerGas: 1500000000n,
      } as never);

      const fees = await provider.getFees();
      expect(fees).toEqual({ maxFeePerGas: 3000000000n, maxPriorityFeePerGas: 1500000000n });
    });
  });
});
