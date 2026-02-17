import { etherToWei, normalizeAddress } from "@consensys/linea-shared-utils";
import { Client, Hex, PrivateKeyAccount } from "viem";
import { privateKeyToAccount, generatePrivateKey, privateKeyToAddress } from "viem/accounts";

import Account from "./account";
import { AccountFundingService } from "./account-funding-service";
import { type TransactionResult } from "../../common/utils/retry";
import { createTestLogger } from "../logger";

import type { Logger } from "winston";

interface IAccountManager {
  whaleAccount(accIndex?: number): PrivateKeyAccount;
  generateAccount(initialBalanceWei?: bigint, accIndex?: number): Promise<PrivateKeyAccount>;
  generateAccounts(
    numberOfAccounts: number,
    initialBalanceWei?: bigint,
    accIndex?: number,
  ): Promise<PrivateKeyAccount[]>;
}

/** Ensures a private key is 0x-prefixed and zero-padded to 32 bytes for viem compatibility. */
function formatPrivateKey(privateKey: string): Hex {
  if (!privateKey.startsWith("0x")) {
    privateKey = "0x" + privateKey;
  }
  let keyWithoutPrefix = privateKey.slice(2);

  // Pad the private key to 64 hex characters (32 bytes) if it's shorter.
  if (keyWithoutPrefix.length < 64) {
    keyWithoutPrefix = keyWithoutPrefix.padStart(64, "0");
  }
  return `0x${keyWithoutPrefix}`;
}

/**
 * Manages whale (pre-funded) accounts and generates fresh test accounts funded from them.
 * Each Jest worker is assigned a distinct whale account to avoid nonce conflicts in parallel test runs.
 */
abstract class AccountManager implements IAccountManager {
  protected readonly chainId: number;
  protected readonly whaleAccounts: Account[];
  protected readonly reservedAddresses: string[];
  protected client: Client;
  protected accountWallets: PrivateKeyAccount[];
  private logger: Logger;
  private fundingService: AccountFundingService;

  constructor(client: Client, whaleAccounts: Account[], chainId: number, reservedAddresses: string[] = []) {
    this.client = client;
    this.whaleAccounts = whaleAccounts;
    this.chainId = chainId;
    this.reservedAddresses = reservedAddresses.map(normalizeAddress);
    this.accountWallets = this.whaleAccounts.map((account) =>
      privateKeyToAccount(formatPrivateKey(account.privateKey)),
    );

    this.logger = createTestLogger();
    this.fundingService = new AccountFundingService(this.client, this.chainId, this.logger);
  }

  /**
   * Selects a whale account for funding operations.
   * If an explicit index is provided, uses that directly.
   * Otherwise, maps the current Jest worker ID to an available (non-reserved) whale account
   * using round-robin assignment to prevent nonce collisions across parallel workers.
   */
  selectWhaleAccount(accIndex?: number): { account: Account; accountWallet: PrivateKeyAccount } {
    if (accIndex !== undefined) {
      return { account: this.whaleAccounts[accIndex], accountWallet: this.accountWallets[accIndex] };
    }

    const workerIdEnv = process.env.JEST_WORKER_ID ?? "1";
    const workerId = Number(workerIdEnv) - 1;

    // Exclude reserved addresses (e.g. accounts used by contracts or other infra).
    const availableWhaleAccounts = this.whaleAccounts.filter(
      (account) => !this.reservedAddresses.includes(normalizeAddress(account.address)),
    );

    if (availableWhaleAccounts.length === 0) {
      throw new Error("No available whale accounts found after filtering reserved addresses.");
    }

    if (workerId >= availableWhaleAccounts.length) {
      this.logger.warn(
        `More Jest workers than available whale accounts. ` +
          `workerId=${workerId} whaleAccounts=${availableWhaleAccounts.length} — ` +
          `multiple workers will share the same whale, risking nonce conflicts.`,
      );
    }

    const isValidWorkerId = Number.isFinite(workerId) && workerId >= 0;
    if (!isValidWorkerId) {
      this.logger.warn(`Invalid JEST_WORKER_ID value. value=${workerIdEnv}`);
    }
    // Round-robin: each worker gets a deterministic whale account.
    const accountIndex = isValidWorkerId ? workerId % availableWhaleAccounts.length : 0;
    const whaleAccount = availableWhaleAccounts[accountIndex];
    const whaleTxManager = this.accountWallets[this.whaleAccounts.indexOf(whaleAccount)];
    return { account: whaleAccount, accountWallet: whaleTxManager };
  }

  whaleAccount(accIndex?: number): PrivateKeyAccount {
    return this.selectWhaleAccount(accIndex).accountWallet;
  }

  async generateAccount(initialBalanceWei = etherToWei("10"), accIndex?: number): Promise<PrivateKeyAccount> {
    const accounts = await this.generateAccounts(1, initialBalanceWei, accIndex);
    return accounts[0];
  }

  /**
   * Generates fresh random accounts and funds each from the selected whale account.
   * All funding transactions are dispatched concurrently, then awaited together.
   * Fails fast if any account is not funded — partial funding would cause unpredictable test failures.
   */
  async generateAccounts(
    numberOfAccounts: number,
    initialBalanceWei = etherToWei("10"),
    accIndex?: number,
  ): Promise<PrivateKeyAccount[]> {
    const { account: whaleAccount, accountWallet: whaleAccountWallet } = this.selectWhaleAccount(accIndex);

    this.logger.debug(
      `Generating accounts... chainId=${this.chainId} numberOfAccounts=${numberOfAccounts} whaleAccount=${whaleAccount.address}`,
    );

    // Step 1: Generate random accounts and kick off funding transactions concurrently.
    const accountTransactionPairs: Array<{ account: Account; txPromise: Promise<TransactionResult | null> }> = [];

    for (let i = 0; i < numberOfAccounts; i++) {
      const randomPrivKey = generatePrivateKey();
      const newAccount = new Account(randomPrivKey, privateKeyToAddress(randomPrivKey));

      const txPromise = this.fundingService.fundAccount(
        whaleAccountWallet,
        whaleAccount.address,
        newAccount.address,
        initialBalanceWei,
      );

      accountTransactionPairs.push({ account: newAccount, txPromise });
    }

    // Step 2: Await all funding transactions (each includes receipt confirmation via sendTransactionWithRetry).
    const transactionResults = await Promise.all(accountTransactionPairs.map((pair) => pair.txPromise));

    const successfulTransactions = transactionResults.filter((result): result is TransactionResult => result !== null);

    // Step 3: Fail fast — all accounts must be funded for tests to be reliable.
    const failedCount = numberOfAccounts - successfulTransactions.length;
    if (failedCount > 0) {
      throw new Error(
        `Failed to fund all accounts. successful=${successfulTransactions.length} failed=${failedCount} expected=${numberOfAccounts}`,
      );
    }

    const allAccounts = accountTransactionPairs.map((pair) => pair.account);

    this.logger.debug(
      `All ${numberOfAccounts} accounts funded successfully. newAccounts=${allAccounts
        .map((account) => account.address)
        .join(", ")} balance=${initialBalanceWei.toString()} Wei`,
    );

    return allAccounts.map((account) => privateKeyToAccount(account.privateKey));
  }
}

export { AccountManager };
