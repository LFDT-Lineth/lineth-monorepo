import { Mutex } from "async-mutex";
import { Address, Client, PrivateKeyAccount } from "viem";
import { estimateFeesPerGas, getTransactionCount, sendTransaction } from "viem/actions";

import { estimateLineaGas, normalizeEip1559Fees, type Eip1559Fees } from "../../common/utils";
import { sendTransactionWithRetry, type TransactionResult } from "../../common/utils/retry";

import type { Logger } from "winston";

type FeeData = Eip1559Fees;

const DEFAULT_RECEIPT_TIMEOUT_MS = 30_000;

/**
 * Sends funding transactions from a whale account to newly generated test accounts.
 * Uses a local nonce counter (protected by a mutex) to assign sequential nonces
 * without holding the lock during receipt confirmation. This allows concurrent
 * in-flight funding transactions from the same whale while preventing nonce collisions.
 */
export class AccountFundingService {
  private readonly nonceMutex = new Mutex();
  private readonly localNonces = new Map<Address, number>();

  constructor(
    private readonly client: Client,
    private readonly chainId: number,
    private readonly logger: Logger,
  ) {}

  /**
   * Funds a single target address from the whale account.
   * Returns the transaction result on success or null if all retry attempts are exhausted.
   * On terminal failure, invalidates the cached nonce so the next call re-fetches from chain.
   */
  async fundAccount(
    whaleAccountWallet: PrivateKeyAccount,
    whaleAccountAddress: Address,
    targetAddress: Address,
    initialBalanceWei: bigint,
  ): Promise<TransactionResult | null> {
    try {
      const feeData = await this.estimateFees(whaleAccountWallet.address, targetAddress, initialBalanceWei);
      const nonce = await this.nextNonce(whaleAccountAddress);

      const result = await sendTransactionWithRetry(
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

      this.logger.debug(
        `Account funded. targetAddress=${targetAddress} txHash=${result.hash} whaleAccount=${whaleAccountAddress}`,
      );

      return result;
    } catch (error) {
      this.logger.error(`Failed to fund account. address=${targetAddress} error=${(error as Error).message}`);
      this.invalidateNonce(whaleAccountAddress);
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

    const feeData = await estimateFeesPerGas(this.client);
    return normalizeEip1559Fees(feeData.maxPriorityFeePerGas, feeData.maxFeePerGas);
  }
}
