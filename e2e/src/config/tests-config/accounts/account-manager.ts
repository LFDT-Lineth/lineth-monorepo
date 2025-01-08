import { ethers, NonceManager, Provider, TransactionResponse, Wallet } from "ethers";
import { Mutex } from "async-mutex";
import type { Logger } from "winston";
import Account from "./account";
import { etherToWei } from "../../../common/utils";
import { createTestLogger } from "../../../config/logger";

interface IAccountManager {
  whaleAccount(accIndex?: number): NonceManager;
  generateAccount(initialBalanceWei?: bigint): Promise<Wallet>;
  generateAccounts(numberOfAccounts: number, initialBalanceWei?: bigint): Promise<Wallet[]>;
  getWallet(account: Account): Wallet;
}

function getWallet(provider: Provider, privateKey: string): Wallet {
  if (!privateKey.startsWith("0x")) {
    privateKey = "0x" + privateKey;
  }
  let keyWithoutPrefix = privateKey.slice(2);

  // Pad the private key to 64 hex characters (32 bytes) if it's shorter
  if (keyWithoutPrefix.length < 64) {
    keyWithoutPrefix = keyWithoutPrefix.padStart(64, "0");
  }
  return new Wallet(`0x${keyWithoutPrefix}`, provider);
}

abstract class AccountManager implements IAccountManager {
  protected readonly chainId: number;
  protected readonly whaleAccounts: Account[];
  protected provider: Provider;
  protected accountWallets: NonceManager[];
  private whaleAccountMutex: Mutex;
  private logger: Logger;

  private readonly MAX_RETRIES = 5;
  private readonly RETRY_DELAY_MS = 1_000;

  constructor(provider: Provider, whaleAccounts: Account[], chainId: number) {
    this.provider = provider;
    this.whaleAccounts = whaleAccounts;
    this.chainId = chainId;
    this.accountWallets = this.whaleAccounts.map(
      (account) => new NonceManager(getWallet(this.provider, account.privateKey)),
    );
    this.whaleAccountMutex = new Mutex();

    this.logger = createTestLogger();
  }

  selectWhaleAccount(accIndex?: number): { account: Account; accountWallet: NonceManager } {
    if (accIndex) {
      return { account: this.whaleAccounts[accIndex], accountWallet: this.accountWallets[accIndex] };
    }
    const workerIdEnv = process.env.JEST_WORKER_ID || "1";
    const workerId = parseInt(workerIdEnv, 10) - 1;

    const accountIndex = workerId;
    const whaleAccount = this.whaleAccounts[accountIndex];
    const whaleTxManager = this.accountWallets[this.whaleAccounts.indexOf(whaleAccount)];
    return { account: whaleAccount, accountWallet: whaleTxManager };
  }

  whaleAccount(accIndex?: number): NonceManager {
    return this.selectWhaleAccount(accIndex).accountWallet;
  }

  async generateAccount(initialBalanceWei = etherToWei("10")): Promise<Wallet> {
    const accounts = await this.generateAccounts(1, initialBalanceWei);
    return accounts[0];
  }

  async generateAccounts(numberOfAccounts: number, initialBalanceWei = etherToWei("10")): Promise<Wallet[]> {
    const { account: whaleAccount, accountWallet: whaleAccountWallet } = this.selectWhaleAccount();

    this.logger.debug(
      `Generating accounts... chainId=${this.chainId} numberOfAccounts=${numberOfAccounts} whaleAccount=${whaleAccount.address}`,
    );

    const accounts: Account[] = [];
    const transactionPromises: Promise<TransactionResponse>[] = [];

    for (let i = 0; i < numberOfAccounts; i++) {
      const randomBytes = ethers.randomBytes(32);
      const randomPrivKey = ethers.hexlify(randomBytes);
      const newAccount = new Account(randomPrivKey, ethers.computeAddress(randomPrivKey));
      accounts.push(newAccount);

      const tx = {
        to: newAccount.address,
        value: initialBalanceWei,
        gasPrice: ethers.parseUnits("300", "gwei"),
        gasLimit: 21000n,
      };

      const sendTransactionWithRetry = async (): Promise<TransactionResponse> => {
        return this.retry<TransactionResponse>(
          async () => {
            const release = await this.whaleAccountMutex.acquire();
            try {
              const transactionResponse = await whaleAccountWallet.sendTransaction(tx);
              this.logger.debug(
                `Transaction sent. newAccount=${newAccount.address} txHash=${transactionResponse.hash} whaleAccount=${whaleAccount.address}`,
              );
              return transactionResponse;
            } catch (error) {
              this.logger.warn(
                `sendTransaction failed for account=${newAccount.address}. Error: ${(error as Error).message}`,
              );
              throw error;
            } finally {
              release();
            }
          },
          this.MAX_RETRIES,
          this.RETRY_DELAY_MS,
        );
      };

      const txPromise = sendTransactionWithRetry().catch((error) => {
        this.logger.error(
          `Failed to fund account after ${this.MAX_RETRIES} attempts. address=${newAccount.address} error=${error.message}`,
        );
        whaleAccountWallet.reset();
        return null as unknown as TransactionResponse;
      });

      transactionPromises.push(txPromise);
    }

    const transactionResponses = await Promise.all(transactionPromises);

    const successfulTransactions = transactionResponses.filter(
      (txResponse): txResponse is TransactionResponse => txResponse !== null,
    );

    if (successfulTransactions.length < numberOfAccounts) {
      this.logger.warn(
        `Some accounts were not funded successfully. successful=${successfulTransactions.length} expected=${numberOfAccounts}`,
      );
    }

    await Promise.all(successfulTransactions.map((tx) => tx.wait()));

    this.logger.debug(
      `${successfulTransactions.length} accounts funded. newAccounts=${accounts
        .map((account) => account.address)
        .join(", ")} balance=${initialBalanceWei.toString()} Wei`,
    );

    return accounts.map((account) => this.getWallet(account));
  }

  getWallet(account: Account): Wallet {
    return getWallet(this.provider, account.privateKey);
  }

  private async retry<T>(fn: () => Promise<T>, retries: number, delayMs: number): Promise<T> {
    let attempt = 0;

    while (attempt < retries) {
      try {
        return await fn();
      } catch (error) {
        attempt++;
        if (attempt >= retries) {
          this.logger.error(`Operation failed after attempts=${attempt} error=${(error as Error).message}`);
          throw error;
        }
        this.logger.warn(`Attempt ${attempt} failed. Retrying in ${delayMs}ms... error=${(error as Error).message}`);
        await this.delay(delayMs);
      }
    }

    throw new Error("Unexpected error in retry mechanism.");
  }

  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}

export { AccountManager };
