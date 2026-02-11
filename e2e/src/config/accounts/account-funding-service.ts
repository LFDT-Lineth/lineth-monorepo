import { Mutex } from "async-mutex";
import { Address, Client, parseGwei, PrivateKeyAccount, SendTransactionReturnType } from "viem";
import { estimateFeesPerGas, sendTransaction } from "viem/actions";

import { RetryPolicy } from "./retry-policy";
import { estimateLineaGas } from "../../common/utils";

import type { Logger } from "winston";

type FeeData = {
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
};

/**
 * Sends funding transactions from a whale account to newly generated test accounts.
 * Uses a mutex to serialize sends from the same whale and avoid nonce races,
 * and a retry policy to handle transient RPC or mempool failures.
 */
export class AccountFundingService {
  private readonly whaleAccountMutex = new Mutex();
  private readonly retryPolicy: RetryPolicy;

  constructor(
    private readonly client: Client,
    private readonly chainId: number,
    private readonly logger: Logger,
    retryOptions: { retries: number; delayMs: number },
  ) {
    this.retryPolicy = new RetryPolicy(logger, retryOptions);
  }

  /**
   * Funds a single target address from the whale account.
   * Returns the tx hash on success or null if all retry attempts are exhausted.
   * On terminal failure, resets the whale's nonce manager to resync with on-chain state.
   */
  async fundAccount(
    whaleAccountWallet: PrivateKeyAccount,
    whaleAccountAddress: Address,
    targetAddress: Address,
    initialBalanceWei: bigint,
  ): Promise<SendTransactionReturnType | null> {
    const feeData = await this.estimateFees(whaleAccountWallet.address, targetAddress, initialBalanceWei);

    const sendWithRetry = async (): Promise<SendTransactionReturnType> => {
      return this.retryPolicy.execute(async () => {
        // Mutex ensures only one transaction is in-flight per whale at a time,
        // preventing nonce collisions when multiple accounts are funded concurrently.
        const release = await this.whaleAccountMutex.acquire();
        try {
          const transactionHash = await sendTransaction(this.client, {
            account: whaleAccountWallet,
            chain: this.client.chain,
            type: "eip1559",
            to: targetAddress,
            value: initialBalanceWei,
            maxPriorityFeePerGas: feeData.maxPriorityFeePerGas,
            maxFeePerGas: feeData.maxFeePerGas,
            gas: 21000n,
          });
          this.logger.debug(
            `Transaction sent. newAccount=${targetAddress} txHash=${transactionHash} whaleAccount=${whaleAccountAddress}`,
          );
          return transactionHash;
        } catch (error) {
          this.logger.warn(`sendTransaction failed for account=${targetAddress}. Error: ${(error as Error).message}`);
          throw error;
        } finally {
          release();
        }
      });
    };

    return sendWithRetry().catch((error) => {
      this.logger.error(
        `Failed to fund account after retries. address=${targetAddress} error=${(error as Error).message}`,
      );
      // Reset nonce manager so subsequent sends don't use a stale nonce.
      whaleAccountWallet.nonceManager?.reset({
        address: whaleAccountWallet.address,
        chainId: this.chainId,
      });
      return null;
    });
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
    return {
      maxPriorityFeePerGas: feeData.maxPriorityFeePerGas ?? parseGwei("1"),
      maxFeePerGas: feeData.maxFeePerGas ?? parseGwei("10"),
    };
  }
}
