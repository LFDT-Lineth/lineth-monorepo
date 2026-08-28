import { Mutex } from "async-mutex";
import { Address, BaseError, Client, PrivateKeyAccount, PublicActions } from "viem";
import { getTransactionCount, sendTransaction } from "viem/actions";

import {
  awaitUntil,
  AwaitUntilTimeoutError,
  createBlockNotFoundRetryExtension,
  estimateLineaGas,
  normalizeEip1559Fees,
  type Eip1559Fees,
} from "../../common/utils";
import { sendTransactionWithRetry, type TransactionResult } from "../../common/utils/retry";

import type { Logger } from "winston";

type FundingClient = Client & Pick<PublicActions, "estimateFeesPerGas">;

type FeeData = Eip1559Fees;

const DEFAULT_RECEIPT_TIMEOUT_MS = 30_000;

// 60 s is less than half the smallest EIP-7702 test timeout (120 s), so a funding failure
// will surface as a clear error message before Jest's deadline rather than a generic
// "test timed out". It is also large enough to outlast brief sequencer instability windows
// (observed ~3 s in CI) with many retries to spare at the 500 ms polling interval.
const FUND_TIMEOUT_MS = 60_000;

// One L2 block (~1 s) is usually enough for mempool pressure from concurrent test setups
// to clear. 500 ms keeps retries responsive while giving the sequencer time to process
// the competing transactions that caused the immediate rejection.
const FUND_RETRY_DELAY_MS = 500;

/**
 * Typed viem error names that indicate a transient RPC / mempool condition.
 * A brief pause and a fresh on-chain nonce are likely to resolve these on the next attempt.
 *
 * - NonceTooLowError         — stale cached nonce; re-fetch and retry
 * - FeeCapTooLowError        — covers "replacement transaction underpriced" / "transaction underpriced"
 * - TipAboveFeeCapError      — fee estimation drift between estimate and submit
 * - TransactionRejectedRpcError — mempool full, already known, general RPC rejection
 * - InternalRpcError         — transient sequencer / node error
 * - LimitExceededRpcError    — pool limits or rate limiting
 */
const TRANSIENT_FUNDING_ERROR_NAMES = new Set([
  "NonceTooLowError",
  "FeeCapTooLowError",
  "TipAboveFeeCapError",
  "TransactionRejectedRpcError",
  "InternalRpcError",
  "LimitExceededRpcError",
]);

/**
 * Returns true when the error (or any cause in its chain) is a known transient
 * RPC / mempool condition. Uses BaseError.walk() to inspect the full cause chain,
 * matching the same pattern used in retry.ts.
 */
function isTransientFundingError(error: unknown): boolean {
  if (!(error instanceof BaseError)) return false;
  return (
    error.walk((cause) => {
      const name = (cause as { name?: string }).name;
      return typeof name === "string" && TRANSIENT_FUNDING_ERROR_NAMES.has(name);
    }) !== null
  );
}

/**
 * Sends funding transactions from a whale account to newly generated test accounts.
 * Uses a local nonce counter (protected by a mutex) to assign sequential nonces
 * without holding the lock during receipt confirmation. This allows concurrent
 * in-flight funding transactions from the same whale while preventing nonce collisions.
 */
export class AccountFundingService {
  private readonly nonceMutex = new Mutex();
  private readonly localNonces = new Map<Address, number>();
  private readonly client: FundingClient;

  constructor(
    client: Client,
    private readonly chainId: number,
    private readonly logger: Logger,
  ) {
    this.client = client.extend(createBlockNotFoundRetryExtension());
  }

  /**
   * Funds a single target address from the whale account.
   * Returns the transaction result on success, or null if the attempt fails.
   *
   * Retries within a FUND_TIMEOUT_MS deadline on transient RPC / mempool errors (e.g.
   * immediate rejection due to nonce pressure or sequencer instability). Each retry
   * invalidates the cached nonce so the next attempt re-fetches from chain.
   * Non-transient errors (reverts, authorization failures, etc.) bail out immediately.
   */
  async fundAccount(
    whaleAccountWallet: PrivateKeyAccount,
    whaleAccountAddress: Address,
    targetAddress: Address,
    initialBalanceWei: bigint,
  ): Promise<TransactionResult | null> {
    try {
      const result = await awaitUntil(
        async () => {
          try {
            const feeData = await this.estimateFees(whaleAccountWallet.address, targetAddress, initialBalanceWei);
            const nonce = await this.nextNonce(whaleAccountAddress);

            return await sendTransactionWithRetry(
              this.client,
              (fees) =>
                sendTransaction(this.client, {
                  account: whaleAccountWallet,
                  chain: this.client.chain,
                  type: "eip1559",
                  to: targetAddress,
                  value: initialBalanceWei,
                  nonce,
                  gas: 21000n,
                  ...feeData,
                  ...fees,
                }),
              { receiptTimeoutMs: DEFAULT_RECEIPT_TIMEOUT_MS },
            );
          } catch (error) {
            this.invalidateNonce(whaleAccountAddress);
            throw error;
          }
        },
        () => true,
        { timeoutMs: FUND_TIMEOUT_MS, pollingIntervalMs: FUND_RETRY_DELAY_MS, shouldRetry: isTransientFundingError },
      );

      this.logger.debug(
        `Account funded. targetAddress=${targetAddress} txHash=${result.hash} whaleAccount=${whaleAccountAddress}`,
      );

      return result;
    } catch (error) {
      if (error instanceof AwaitUntilTimeoutError) {
        this.logger.error(`Failed to fund account: timeout after ${error.timeoutMs}ms. address=${targetAddress}`);
      } else {
        this.logger.error(`Failed to fund account. address=${targetAddress} error=${(error as Error).message}`);
      }
      return null;
    }
  }

  /**
   * Assigns the next nonce for the given whale address.
   * On first call (or after invalidation), fetches the pending nonce from chain;
   * subsequent calls increment a local counter, allowing concurrent funding
   * without holding a lock during receipt confirmation.
   */
  private async nextNonce(address: Address): Promise<number> {
    const release = await this.nonceMutex.acquire();
    try {
      let nonce = this.localNonces.get(address);
      if (nonce === undefined) {
        nonce = await getTransactionCount(this.client, { address, blockTag: "pending" });
      }
      this.localNonces.set(address, nonce + 1);
      return nonce;
    } finally {
      release();
    }
  }

  /**
   * Clears the cached nonce for the given address so the next call re-fetches from chain.
   * Called on funding failure to resync with on-chain state.
   */
  private invalidateNonce(address: Address): void {
    this.localNonces.delete(address);
  }

  /**
   * Estimates EIP-1559 fee parameters.
   * Uses the Linea-specific gas oracle for the local dev chain (chainId 1337),
   * and the standard viem fee estimator for all other networks with safe defaults as fallback.
   */
  private async estimateFees(fromAddress: Address, toAddress: Address, value: bigint): Promise<FeeData> {
    if (this.chainId === 1337) {
      const feeData = await estimateLineaGas(this.client, {
        account: fromAddress,
        to: toAddress,
        value,
      });
      return {
        maxPriorityFeePerGas: feeData.maxPriorityFeePerGas,
        maxFeePerGas: feeData.maxFeePerGas,
      };
    }

    const feeData = await this.client.estimateFeesPerGas();
    return normalizeEip1559Fees(feeData.maxPriorityFeePerGas, feeData.maxFeePerGas);
  }
}
