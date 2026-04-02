import { ILogger } from "@consensys/linea-shared-utils";

import { ITransactionCountProvider } from "../../../../core/clients/blockchain/IProvider";
import { IDbNonceProvider } from "../../../../core/services/IDbNonceProvider";
import { INonceManager } from "../../../../core/services/INonceManager";
import { Address } from "../../../../core/types";

export class NonceManager implements INonceManager {
  private nextNonce = 0;
  private readonly reusable: number[] = [];
  private readonly pending = new Set<number>();
  private readonly maxNonceDiff: number;

  private locked = false;
  private readonly waitQueue: Array<(value: void) => void> = [];

  constructor(
    private readonly provider: ITransactionCountProvider,
    private readonly dbNonceProvider: IDbNonceProvider,
    private readonly signerAddress: Address,
    maxNonceDiff: number,
    private readonly logger: ILogger,
  ) {
    this.maxNonceDiff = Math.max(maxNonceDiff, 0);
  }

  public async initialize(): Promise<void> {
    const [onChainNonce, dbMaxNonce] = await Promise.all([
      this.provider.getTransactionCount(this.signerAddress, "pending"),
      this.dbNonceProvider.getMaxPendingNonce(),
    ]);

    this.nextNonce = dbMaxNonce !== null ? Math.max(onChainNonce, dbMaxNonce + 1) : onChainNonce;

    this.logger.info("NonceManager initialized.", {
      startNonce: this.nextNonce,
      onChainNonce,
      dbMaxNonce,
    });
  }

  public async acquireNonce(): Promise<number> {
    await this.lock();
    try {
      const [onChainNonce, dbMaxNonce] = await Promise.all([
        this.provider.getTransactionCount(this.signerAddress, "pending"),
        this.dbNonceProvider.getMaxPendingNonce(),
      ]);

      this.pruneConfirmed(onChainNonce);

      const effectiveFloor = dbMaxNonce !== null ? Math.max(onChainNonce, dbMaxNonce + 1) : onChainNonce;
      const drift = this.nextNonce - effectiveFloor;

      if (drift > this.maxNonceDiff && this.reusable.length === 0) {
        this.logger.warn("Nonce drift exceeds limit, pausing claiming until pending transactions confirm.", {
          nextNonce: this.nextNonce,
          onChainNonce,
          dbMaxNonce,
          effectiveFloor,
          maxNonceDiff: this.maxNonceDiff,
          pendingCount: this.pending.size,
        });
        throw new Error(
          `Nonce drift ${drift} exceeds max allowed ${this.maxNonceDiff}. ` +
            `Waiting for in-flight transactions to confirm before issuing new nonces.`,
        );
      }

      if (this.nextNonce < effectiveFloor) {
        this.nextNonce = effectiveFloor;
      }

      let nonce: number;
      if (this.reusable.length > 0) {
        nonce = this.reusable.shift()!;
      } else {
        nonce = this.nextNonce++;
      }

      this.pending.add(nonce);

      this.logger.debug("Nonce acquired.", {
        nonce,
        nextNonce: this.nextNonce,
        pendingCount: this.pending.size,
        reusableCount: this.reusable.length,
      });

      return nonce;
    } finally {
      this.unlock();
    }
  }

  public commitNonce(nonce: number): void {
    this.pending.delete(nonce);
    this.logger.debug("Nonce committed.", { nonce, pendingCount: this.pending.size });
  }

  public rollbackNonce(nonce: number): void {
    this.pending.delete(nonce);
    const idx = this.reusable.findIndex((n) => n > nonce);
    if (idx === -1) {
      this.reusable.push(nonce);
    } else {
      this.reusable.splice(idx, 0, nonce);
    }
    this.logger.debug("Nonce rolled back.", { nonce, reusableCount: this.reusable.length });
  }

  /**
   * Removes nonces that the chain has already consumed (confirmed or in mempool).
   * Prevents reuse of nonces that were rolled back locally but actually submitted
   * on-chain (e.g. when a tx submission succeeds but the RPC call times out).
   */
  private pruneConfirmed(onChainNonce: number): void {
    const prunedReusable: number[] = [];
    while (this.reusable.length > 0 && this.reusable[0] < onChainNonce) {
      prunedReusable.push(this.reusable.shift()!);
    }

    const prunedPending: number[] = [];
    for (const nonce of this.pending) {
      if (nonce < onChainNonce) {
        prunedPending.push(nonce);
      }
    }
    for (const nonce of prunedPending) {
      this.pending.delete(nonce);
    }

    if (prunedReusable.length > 0 || prunedPending.length > 0) {
      this.logger.info("Pruned confirmed nonces.", {
        prunedReusable,
        prunedPending,
        onChainNonce,
      });
    }
  }

  private async lock(): Promise<void> {
    if (!this.locked) {
      this.locked = true;
      return;
    }
    return new Promise<void>((resolve) => {
      this.waitQueue.push(resolve);
    });
  }

  private unlock(): void {
    const next = this.waitQueue.shift();
    if (next) {
      next();
    } else {
      this.locked = false;
    }
  }
}
