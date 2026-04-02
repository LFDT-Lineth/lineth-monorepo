import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { ITransactionCountProvider } from "../../../../../core/clients/blockchain/IProvider";
import { IDbNonceProvider } from "../../../../../core/services/IDbNonceProvider";
import { TEST_SIGNER_ADDRESS } from "../../../../../utils/testing/constants";
import { TestLogger } from "../../../../../utils/testing/helpers";
import { NonceManager } from "../NonceManager";

describe("NonceManager", () => {
  let provider: ReturnType<typeof mock<ITransactionCountProvider>>;
  let dbNonceProvider: ReturnType<typeof mock<IDbNonceProvider>>;
  let logger: TestLogger;

  beforeEach(() => {
    provider = mock<ITransactionCountProvider>();
    dbNonceProvider = mock<IDbNonceProvider>();
    logger = new TestLogger("NonceManager");
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  function buildManager(maxNonceDiff = 5) {
    return new NonceManager(provider, dbNonceProvider, TEST_SIGNER_ADDRESS, maxNonceDiff, logger);
  }

  describe("initialize", () => {
    it("sets nextNonce from on-chain nonce when no DB nonces exist", async () => {
      provider.getTransactionCount.mockResolvedValue(7);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      provider.getTransactionCount.mockResolvedValue(7);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(7);
    });

    it("sets nextNonce from DB max nonce + 1 when it exceeds on-chain nonce", async () => {
      provider.getTransactionCount.mockResolvedValue(5);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(9);
      const manager = buildManager();
      await manager.initialize();

      provider.getTransactionCount.mockResolvedValue(5);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(9);
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(10);
    });

    it("uses on-chain nonce when it exceeds DB max nonce + 1", async () => {
      provider.getTransactionCount.mockResolvedValue(15);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(9);
      const manager = buildManager();
      await manager.initialize();

      provider.getTransactionCount.mockResolvedValue(15);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(9);
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(15);
    });

    it("queries the correct signer address with pending block tag", async () => {
      provider.getTransactionCount.mockResolvedValue(0);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      expect(provider.getTransactionCount).toHaveBeenCalledWith(TEST_SIGNER_ADDRESS, "pending");
    });
  });

  describe("acquireNonce", () => {
    it("returns sequential nonces", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const n1 = await manager.acquireNonce();
      const n2 = await manager.acquireNonce();
      const n3 = await manager.acquireNonce();

      expect(n1).toBe(10);
      expect(n2).toBe(11);
      expect(n3).toBe(12);
    });

    it("advances nextNonce when DB nonce catches up", async () => {
      provider.getTransactionCount.mockResolvedValue(5);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const n1 = await manager.acquireNonce();
      expect(n1).toBe(5);

      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(7);
      const n2 = await manager.acquireNonce();
      expect(n2).toBe(8);
    });

    it("throws when drift exceeds maxNonceDiff to pause claiming", async () => {
      provider.getTransactionCount.mockResolvedValue(5);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager(2);
      await manager.initialize();

      await manager.acquireNonce(); // 5
      await manager.acquireNonce(); // 6
      await manager.acquireNonce(); // 7
      manager.commitNonce(5);
      manager.commitNonce(6);
      manager.commitNonce(7);

      // nextNonce=8, onChain=5, drift=3 > maxNonceDiff=2
      const warnSpy = jest.spyOn(logger, "warn");
      await expect(manager.acquireNonce()).rejects.toThrow("Nonce drift 3 exceeds max allowed 2");
      expect(warnSpy).toHaveBeenCalledTimes(1);
    });

    it("resumes after drift resolves", async () => {
      provider.getTransactionCount.mockResolvedValue(5);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager(2);
      await manager.initialize();

      await manager.acquireNonce(); // 5
      await manager.acquireNonce(); // 6
      await manager.acquireNonce(); // 7
      manager.commitNonce(5);
      manager.commitNonce(6);
      manager.commitNonce(7);

      // Drift too high — pauses
      await expect(manager.acquireNonce()).rejects.toThrow("Nonce drift");

      // On-chain catches up → drift within limit
      provider.getTransactionCount.mockResolvedValue(7);
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(8);
    });

    it("does not pause when reusable nonces are available despite high drift", async () => {
      provider.getTransactionCount.mockResolvedValue(5);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager(2);
      await manager.initialize();

      const n1 = await manager.acquireNonce(); // 5
      const n2 = await manager.acquireNonce(); // 6
      const n3 = await manager.acquireNonce(); // 7, nextNonce=8
      manager.commitNonce(n2);
      manager.commitNonce(n3);
      manager.rollbackNonce(n1); // 5 goes back to reusable

      // drift = 8 - 5 = 3 > maxNonceDiff(2), but reusable has nonce 5
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(5);
    });
  });

  describe("commitNonce", () => {
    it("removes nonce from pending set", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const nonce = await manager.acquireNonce();
      manager.commitNonce(nonce);

      expect(nonce).toBe(10);
    });
  });

  describe("rollbackNonce", () => {
    it("makes nonce available for reuse", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const n1 = await manager.acquireNonce();
      await manager.acquireNonce();
      manager.rollbackNonce(n1);

      const reused = await manager.acquireNonce();
      expect(reused).toBe(10);
    });

    it("reuses lowest rolled-back nonce first", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const n1 = await manager.acquireNonce();
      const n2 = await manager.acquireNonce();
      await manager.acquireNonce();

      manager.rollbackNonce(n2);
      manager.rollbackNonce(n1);

      const r1 = await manager.acquireNonce();
      const r2 = await manager.acquireNonce();
      expect(r1).toBe(10);
      expect(r2).toBe(11);
    });
  });

  describe("pruneConfirmed", () => {
    it("removes rolled-back nonces that were consumed on-chain", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const n1 = await manager.acquireNonce(); // 10
      await manager.acquireNonce(); // 11

      // Simulate: tx with nonce 10 was actually submitted but RPC timed out,
      // so the caller rolled it back. The tx is now in the mempool.
      manager.rollbackNonce(n1); // 10 goes to reusable

      // On-chain advances past 10 (tx confirmed or in mempool)
      provider.getTransactionCount.mockResolvedValue(11);

      // Next acquire should NOT reuse 10 — it's been consumed on-chain
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(12);
    });

    it("prunes multiple stale reusable nonces", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const n1 = await manager.acquireNonce(); // 10
      const n2 = await manager.acquireNonce(); // 11
      await manager.acquireNonce(); // 12

      manager.rollbackNonce(n1); // reusable: [10]
      manager.rollbackNonce(n2); // reusable: [10, 11]

      // Chain advanced past both
      provider.getTransactionCount.mockResolvedValue(12);
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(13);
    });

    it("prunes stale pending entries for accurate monitoring", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      await manager.acquireNonce(); // 10, in pending
      await manager.acquireNonce(); // 11, in pending

      // Chain confirms both (without us calling commit — simulates external resolution)
      provider.getTransactionCount.mockResolvedValue(12);

      const infoSpy = jest.spyOn(logger, "info");
      await manager.acquireNonce(); // 12, prunes 10 and 11 from pending

      expect(infoSpy).toHaveBeenCalledWith(
        "Pruned confirmed nonces.",
        expect.objectContaining({ prunedPending: expect.arrayContaining([10, 11]) }),
      );
    });

    it("keeps reusable nonces that are above on-chain nonce", async () => {
      provider.getTransactionCount.mockResolvedValue(10);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const n1 = await manager.acquireNonce(); // 10
      await manager.acquireNonce(); // 11, nextNonce=12

      manager.rollbackNonce(n1); // reusable: [10]

      // Chain hasn't advanced — nonce 10 was genuinely not used
      const nonce = await manager.acquireNonce();
      expect(nonce).toBe(10); // correctly reused
    });
  });

  describe("concurrency", () => {
    it("serializes concurrent acquireNonce calls", async () => {
      provider.getTransactionCount.mockResolvedValue(0);
      dbNonceProvider.getMaxPendingNonce.mockResolvedValue(null);
      const manager = buildManager();
      await manager.initialize();

      const results = await Promise.all([manager.acquireNonce(), manager.acquireNonce(), manager.acquireNonce()]);

      expect(new Set(results).size).toBe(3);
      expect(results.sort()).toEqual([0, 1, 2]);
    });
  });
});
