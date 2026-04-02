import { ILogger } from "@consensys/linea-shared-utils";
import { describe, it, beforeEach, expect } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { type LocalAccount, type PublicClient, type WalletClient } from "viem";

import { TEST_ADDRESS_1, TEST_TRANSACTION_HASH } from "../../../../utils/testing/constants";
import { ViemTransactionRetrier } from "../ViemTransactionRetrier";

const MAX_FEE_CAP = 100_000_000_000n;

describe("ViemTransactionRetrier", () => {
  const publicClient = mock<PublicClient>();
  const walletClient = mock<WalletClient>();
  const logger = mock<ILogger>();
  const account = { address: TEST_ADDRESS_1 } as LocalAccount;
  let retrier: ViemTransactionRetrier;

  beforeEach(() => {
    jest.resetAllMocks();
    retrier = new ViemTransactionRetrier(publicClient, walletClient, account, MAX_FEE_CAP, logger);
  });

  describe("retryWithHigherFee", () => {
    it("throws when transaction not found", async () => {
      publicClient.getTransaction.mockResolvedValue(
        null as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>,
      );
      await expect(retrier.retryWithHigherFee(TEST_TRANSACTION_HASH, 1)).rejects.toThrow("not found");
    });

    it("applies exponential bump (attempt 1 = +10%)", async () => {
      publicClient.getTransaction.mockResolvedValue({
        from: TEST_ADDRESS_1,
        nonce: 5,
        gas: 100_000n,
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 100n,
        to: TEST_ADDRESS_1,
        value: 0n,
        input: "0x",
      } as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xretryhash");

      const result = await retrier.retryWithHigherFee(TEST_TRANSACTION_HASH, 1);

      expect(result.hash).toBe("0xretryhash");
      expect(result.nonce).toBe(5);
      expect(result.maxFeePerGas).toBe(1100n); // 1000 * 110 / 100
      expect(result.maxPriorityFeePerGas).toBe(110n); // 100 * 110 / 100
    });

    it("applies exponential bump (attempt 2 = +20%)", async () => {
      publicClient.getTransaction.mockResolvedValue({
        from: TEST_ADDRESS_1,
        nonce: 5,
        gas: 100_000n,
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 100n,
        to: TEST_ADDRESS_1,
        value: 0n,
        input: "0x",
      } as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xretryhash");

      const result = await retrier.retryWithHigherFee(TEST_TRANSACTION_HASH, 2);

      expect(result.maxFeePerGas).toBe(1200n); // 1000 * 120 / 100
      expect(result.maxPriorityFeePerGas).toBe(120n); // 100 * 120 / 100
    });

    it("applies exponential bump (attempt 3 = +40%)", async () => {
      publicClient.getTransaction.mockResolvedValue({
        from: TEST_ADDRESS_1,
        nonce: 5,
        gas: 100_000n,
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 100n,
        to: TEST_ADDRESS_1,
        value: 0n,
        input: "0x",
      } as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xretryhash");

      const result = await retrier.retryWithHigherFee(TEST_TRANSACTION_HASH, 3);

      expect(result.maxFeePerGas).toBe(1400n); // 1000 * 140 / 100
      expect(result.maxPriorityFeePerGas).toBe(140n); // 100 * 140 / 100
    });

    it("caps fees at maxFeePerGasCap", async () => {
      const smallCap = 500n;
      retrier = new ViemTransactionRetrier(publicClient, walletClient, account, smallCap, logger);

      publicClient.getTransaction.mockResolvedValue({
        from: TEST_ADDRESS_1,
        nonce: 5,
        gas: 100_000n,
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 1000n,
        to: TEST_ADDRESS_1,
        value: 0n,
        input: "0x",
      } as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xretryhash");

      const result = await retrier.retryWithHigherFee(TEST_TRANSACTION_HASH, 1);

      expect(result.maxFeePerGas).toBe(500n);
      expect(result.maxPriorityFeePerGas).toBe(500n);
    });

    it("passes undefined to when transaction.to is null", async () => {
      publicClient.getTransaction.mockResolvedValue({
        from: TEST_ADDRESS_1,
        nonce: 5,
        gas: 100_000n,
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 100n,
        to: null,
        value: 0n,
        input: "0x",
      } as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xretryhash");

      await retrier.retryWithHigherFee(TEST_TRANSACTION_HASH, 1);

      expect(walletClient.sendTransaction).toHaveBeenCalledWith(expect.objectContaining({ to: undefined }));
    });

    it("fetches current fees when tx lacks maxFeePerGas", async () => {
      publicClient.getTransaction.mockResolvedValue({
        from: TEST_ADDRESS_1,
        nonce: 5,
        gas: 100_000n,
        maxFeePerGas: null,
        maxPriorityFeePerGas: null,
        to: TEST_ADDRESS_1,
        value: 0n,
        input: "0x",
      } as unknown as Awaited<ReturnType<PublicClient["getTransaction"]>>);
      publicClient.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 2000n,
        maxPriorityFeePerGas: 200n,
      } as Awaited<ReturnType<PublicClient["estimateFeesPerGas"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xretryhash");

      const result = await retrier.retryWithHigherFee(TEST_TRANSACTION_HASH, 1);

      // Aggressive fees: estimated * 2, capped
      expect(result.maxFeePerGas).toBe(4000n);
      expect(result.maxPriorityFeePerGas).toBe(400n);
    });
  });

  describe("cancelTransaction", () => {
    it("caps aggressive fees at maxFeePerGasCap", async () => {
      const smallCap = 500n;
      retrier = new ViemTransactionRetrier(publicClient, walletClient, account, smallCap, logger);

      publicClient.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 1000n,
      } as Awaited<ReturnType<PublicClient["estimateFeesPerGas"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xcancelhash");

      await retrier.cancelTransaction(10);

      expect(walletClient.sendTransaction).toHaveBeenCalledWith(
        expect.objectContaining({
          maxFeePerGas: 500n,
          maxPriorityFeePerGas: 500n,
        }),
      );
    });

    it("sends a zero-value self-transfer at given nonce", async () => {
      publicClient.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 100n,
      } as Awaited<ReturnType<PublicClient["estimateFeesPerGas"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xcancelhash");

      const hash = await retrier.cancelTransaction(42);

      expect(hash).toBe("0xcancelhash");
      expect(walletClient.sendTransaction).toHaveBeenCalledWith({
        account,
        to: TEST_ADDRESS_1,
        value: 0n,
        data: "0x",
        nonce: 42,
        gas: 21_000n,
        maxFeePerGas: 2000n,
        maxPriorityFeePerGas: 200n,
        chain: null,
      });
    });

    it("uses 110% of stuckFees when stuckBumped exceeds aggressive fees", async () => {
      // aggressive: estimateFeesPerGas * 2 = 1000n, 100n
      publicClient.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 500n,
        maxPriorityFeePerGas: 50n,
      } as Awaited<ReturnType<PublicClient["estimateFeesPerGas"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xcancelhash");

      // stuckBumped: 2000 * 110 / 100 = 2200, 200 * 110 / 100 = 220 — both beat aggressive
      await retrier.cancelTransaction(10, { maxFeePerGas: 2000n, maxPriorityFeePerGas: 200n });

      expect(walletClient.sendTransaction).toHaveBeenCalledWith(
        expect.objectContaining({ maxFeePerGas: 2200n, maxPriorityFeePerGas: 220n }),
      );
    });

    it("uses aggressive fees when they exceed 110% of stuckFees", async () => {
      // aggressive: estimateFeesPerGas * 2 = 2000n, 200n
      publicClient.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 1000n,
        maxPriorityFeePerGas: 100n,
      } as Awaited<ReturnType<PublicClient["estimateFeesPerGas"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xcancelhash");

      // stuckBumped: 500 * 110 / 100 = 550, 50 * 110 / 100 = 55 — both beaten by aggressive
      await retrier.cancelTransaction(10, { maxFeePerGas: 500n, maxPriorityFeePerGas: 50n });

      expect(walletClient.sendTransaction).toHaveBeenCalledWith(
        expect.objectContaining({ maxFeePerGas: 2000n, maxPriorityFeePerGas: 200n }),
      );
    });

    it("caps stuckBumped fees at maxFeePerGasCap", async () => {
      const smallCap = 1000n;
      retrier = new ViemTransactionRetrier(publicClient, walletClient, account, smallCap, logger);

      // aggressive: 100 * 2 = 200n — well below cap and stuckBumped
      publicClient.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 100n,
      } as Awaited<ReturnType<PublicClient["estimateFeesPerGas"]>>);
      walletClient.sendTransaction.mockResolvedValue("0xcancelhash");

      // stuckBumped: 2000 * 110 / 100 = 2200 — exceeds cap of 1000
      await retrier.cancelTransaction(10, { maxFeePerGas: 2000n, maxPriorityFeePerGas: 2000n });

      expect(walletClient.sendTransaction).toHaveBeenCalledWith(
        expect.objectContaining({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 1000n }),
      );
    });
  });
});
