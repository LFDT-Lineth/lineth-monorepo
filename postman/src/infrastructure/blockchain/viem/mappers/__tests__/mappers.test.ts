import { describe, it, expect } from "@jest/globals";

import { TEST_ADDRESS_1, TEST_BLOCK_HASH, TEST_TRANSACTION_HASH } from "../../../../../utils/testing/constants";
import { mapViemReceiptToCoreReceipt, mapViemBlockToCoreBlock, mapViemTransactionToCoreSubmission } from "../index";

import type { GetTransactionReceiptReturnType, GetTransactionReturnType } from "viem";

describe("mapViemReceiptToCoreReceipt", () => {
  it("maps a receipt with all log fields present", () => {
    const receipt = {
      transactionHash: TEST_TRANSACTION_HASH,
      blockNumber: 100n,
      status: "success" as const,
      gasUsed: 21000n,
      effectiveGasPrice: 1000n,
      logs: [
        {
          address: TEST_ADDRESS_1,
          topics: ["0xaabb"] as [`0x${string}`],
          data: "0xccdd" as `0x${string}`,
          blockNumber: 100n,
          transactionHash: TEST_TRANSACTION_HASH,
          logIndex: 3,
        },
      ],
    } as unknown as GetTransactionReceiptReturnType;

    const result = mapViemReceiptToCoreReceipt(receipt);

    expect(result.hash).toBe(TEST_TRANSACTION_HASH);
    expect(result.blockNumber).toBe(100);
    expect(result.status).toBe("success");
    expect(result.gasUsed).toBe(21000n);
    expect(result.gasPrice).toBe(1000n);
    expect(result.logs).toHaveLength(1);
    expect(result.logs[0].transactionHash).toBe(TEST_TRANSACTION_HASH);
    expect(result.logs[0].logIndex).toBe(3);
  });

  it("maps reverted status", () => {
    const receipt = {
      transactionHash: TEST_TRANSACTION_HASH,
      blockNumber: 100n,
      status: "reverted" as const,
      gasUsed: 21000n,
      effectiveGasPrice: 1000n,
      logs: [],
    } as unknown as GetTransactionReceiptReturnType;

    const result = mapViemReceiptToCoreReceipt(receipt);
    expect(result.status).toBe("reverted");
  });

  it("defaults transactionHash to 0x and logIndex to 0 when null", () => {
    const receipt = {
      transactionHash: TEST_TRANSACTION_HASH,
      blockNumber: 100n,
      status: "success" as const,
      gasUsed: 21000n,
      effectiveGasPrice: 1000n,
      logs: [
        {
          address: TEST_ADDRESS_1,
          topics: [] as `0x${string}`[],
          data: "0x" as `0x${string}`,
          blockNumber: 100n,
          transactionHash: null,
          logIndex: null,
        },
      ],
    } as unknown as GetTransactionReceiptReturnType;

    const result = mapViemReceiptToCoreReceipt(receipt);

    expect(result.logs[0].transactionHash).toBe("0x");
    expect(result.logs[0].logIndex).toBe(0);
  });
});

describe("mapViemBlockToCoreBlock", () => {
  it("maps a block with all fields present", () => {
    const result = mapViemBlockToCoreBlock({
      number: 42n,
      timestamp: 1700000000n,
      hash: TEST_BLOCK_HASH,
    });

    expect(result.number).toBe(42);
    expect(result.timestamp).toBe(1700000000);
    expect(result.hash).toBe(TEST_BLOCK_HASH);
  });

  it("defaults number to 0 and hash to 0x when null", () => {
    const result = mapViemBlockToCoreBlock({
      number: null,
      timestamp: 1700000000n,
      hash: null,
    });

    expect(result.number).toBe(0);
    expect(result.hash).toBe("0x");
  });
});

describe("mapViemTransactionToCoreSubmission", () => {
  it("maps a transaction with all EIP-1559 fields present", () => {
    const tx = {
      hash: TEST_TRANSACTION_HASH,
      nonce: 5,
      gas: 100000n,
      maxFeePerGas: 2000n,
      maxPriorityFeePerGas: 200n,
    } as unknown as GetTransactionReturnType;

    const result = mapViemTransactionToCoreSubmission(tx);

    expect(result.hash).toBe(TEST_TRANSACTION_HASH);
    expect(result.nonce).toBe(5);
    expect(result.gasLimit).toBe(100000n);
    expect(result.maxFeePerGas).toBe(2000n);
    expect(result.maxPriorityFeePerGas).toBe(200n);
  });

  it("sets maxFeePerGas and maxPriorityFeePerGas to undefined when null", () => {
    const tx = {
      hash: TEST_TRANSACTION_HASH,
      nonce: 5,
      gas: 100000n,
      maxFeePerGas: null,
      maxPriorityFeePerGas: null,
    } as unknown as GetTransactionReturnType;

    const result = mapViemTransactionToCoreSubmission(tx);

    expect(result.maxFeePerGas).toBeUndefined();
    expect(result.maxPriorityFeePerGas).toBeUndefined();
  });
});
