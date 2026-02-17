import { Client, Hash, TransactionReceipt } from "viem";
import {
  waitForTransactionReceipt,
  getTransactionReceipt,
  getTransaction,
  SendTransactionErrorType,
  WaitForTransactionReceiptErrorType,
} from "viem/actions";

import { createTestLogger } from "../../config/logger";

const DEFAULT_RECEIPT_TIMEOUT_MS = 30_000;
const DEFAULT_FEE_BUMP_FACTOR = 115n;
const DEFAULT_MAX_RETRIES = 20;
const DEFAULT_OVERALL_TIMEOUT_MS = 3 * 60_000;

export type FeeOverrides = {
  maxPriorityFeePerGas: bigint | undefined;
  maxFeePerGas: bigint | undefined;
};

export type SendTransactionWithRetryOptions = {
  receiptTimeoutMs?: number;
  feeBumpFactor?: bigint;
  maxRetries?: number;
  overallTimeoutMs?: number;
  rejectOnRevert?: boolean;
};

export type TransactionResult = {
  hash: Hash;
  receipt: TransactionReceipt;
};

const logger = createTestLogger();

/* ---------------- helpers ---------------- */

function isReceiptTimeout(error: unknown): boolean {
  return (error as WaitForTransactionReceiptErrorType)?.name === "WaitForTransactionReceiptTimeoutError";
}

function isNonceTooLow(error: unknown): boolean {
  const e = error as SendTransactionErrorType;
  return e?.name === "TransactionExecutionError" && e?.cause?.name === "NonceTooLowError";
}

async function safeGetReceipt(client: Client, hash: Hash): Promise<TransactionReceipt | undefined> {
  try {
    return await getTransactionReceipt(client, { hash });
  } catch {
    return undefined;
  }
}

function bumpFeesFromTx(
  tx: { maxFeePerGas: bigint | undefined; maxPriorityFeePerGas: bigint | undefined },
  bumpFactor: bigint,
): FeeOverrides {
  if (tx.maxFeePerGas === undefined || tx.maxPriorityFeePerGas === undefined) {
    throw new Error("sendTransactionWithRetry: non EIP-1559 transaction");
  }

  return {
    maxFeePerGas: (tx.maxFeePerGas * bumpFactor) / 100n,
    maxPriorityFeePerGas: (tx.maxPriorityFeePerGas * bumpFactor) / 100n,
  };
}

function assertReceiptSuccess(hash: Hash, receipt: TransactionReceipt, rejectOnRevert: boolean): void {
  if (rejectOnRevert && receipt.status === "reverted") {
    throw new Error(`Transaction reverted: hash=${hash} blockNumber=${receipt.blockNumber}`);
  }
}

/* ---------------- final wrapper ---------------- */

export async function sendTransactionWithRetry(
  client: Client,
  sendFn: (fees?: FeeOverrides) => Promise<Hash>,
  options: SendTransactionWithRetryOptions = {},
): Promise<TransactionResult> {
  const receiptTimeoutMs = options.receiptTimeoutMs ?? DEFAULT_RECEIPT_TIMEOUT_MS;
  const feeBumpFactor = options.feeBumpFactor ?? DEFAULT_FEE_BUMP_FACTOR;
  const maxRetries = options.maxRetries ?? DEFAULT_MAX_RETRIES;
  const overallTimeoutMs = options.overallTimeoutMs ?? DEFAULT_OVERALL_TIMEOUT_MS;
  const rejectOnRevert = options.rejectOnRevert ?? true;

  const startedAt = Date.now();

  let lastHash = await sendFn();

  const { maxPriorityFeePerGas, maxFeePerGas } = await getTransaction(client, { hash: lastHash });
  let fees = { maxPriorityFeePerGas, maxFeePerGas };
  let attempt = 0;

  logger.debug(
    `tx sent hash=${lastHash} maxFeePerGas=${fees.maxFeePerGas} maxPriorityFeePerGas=${fees.maxPriorityFeePerGas}`,
  );

  while (attempt <= maxRetries) {
    /* ---------- hard deadline ---------- */
    if (Date.now() - startedAt > overallTimeoutMs) {
      logger.debug(`overall timeout exceeded hash=${lastHash} attempt=${attempt}; probing receipt`);

      const txReceipt = await safeGetReceipt(client, lastHash);
      if (txReceipt) {
        logger.debug(`tx confirmed during final probe hash=${lastHash} blockNumber=${txReceipt.blockNumber}`);
        assertReceiptSuccess(lastHash, txReceipt, rejectOnRevert);
        return { hash: lastHash, receipt: txReceipt };
      }

      throw new Error("sendTransactionWithRetry: overall timeout exceeded");
    }

    /* ---------- primary wait ---------- */
    try {
      logger.debug(`waiting for receipt hash=${lastHash} attempt=${attempt} timeoutMs=${receiptTimeoutMs}`);

      const receipt = await waitForTransactionReceipt(client, {
        hash: lastHash,
        timeout: receiptTimeoutMs,
      });

      logger.debug(`tx confirmed hash=${lastHash} blockNumber=${receipt.blockNumber} status=${receipt.status}`);

      assertReceiptSuccess(lastHash, receipt, rejectOnRevert);
      return { hash: lastHash, receipt };
    } catch (err) {
      if (!isReceiptTimeout(err)) throw err;

      logger.debug(`receipt timeout for hash=${lastHash} attempt=${attempt}`);
    }

    /* ---------- race guard ---------- */
    const raceConditionReceipt = await safeGetReceipt(client, lastHash);
    if (raceConditionReceipt) {
      logger.debug(`tx mined during timeout race hash=${lastHash} blockNumber=${raceConditionReceipt.blockNumber}`);
      assertReceiptSuccess(lastHash, raceConditionReceipt, rejectOnRevert);
      return { hash: lastHash, receipt: raceConditionReceipt };
    }

    if (attempt === maxRetries) {
      logger.debug(`max retries reached hash=${lastHash} attempt=${attempt}; probing receipt`);

      const receipt = await safeGetReceipt(client, lastHash);
      if (receipt) {
        assertReceiptSuccess(lastHash, receipt, rejectOnRevert);
        return { hash: lastHash, receipt: receipt };
      }

      throw new Error("sendTransactionWithRetry: max retries exceeded");
    }

    /* ---------- bump from actual tx ---------- */
    const nextFees = bumpFeesFromTx(fees, feeBumpFactor);

    logger.debug(
      `bumping fees hash=${lastHash} attempt=${attempt + 1} ` +
        `prevMaxFeePerGas=${fees.maxFeePerGas} nextMaxFeePerGas=${nextFees.maxFeePerGas} ` +
        `prevMaxPriorityFeePerGas=${fees.maxPriorityFeePerGas} nextMaxPriorityFeePerGas=${nextFees.maxPriorityFeePerGas}`,
    );

    fees = nextFees;
    attempt++;

    try {
      lastHash = await sendFn(fees);

      logger.debug(`replacement tx sent hash=${lastHash} attempt=${attempt}`);
    } catch (sendError) {
      if (isNonceTooLow(sendError)) {
        logger.debug(`nonce too low while retrying hash=${lastHash}; original tx likely mined`);

        const receipt = await safeGetReceipt(client, lastHash);
        if (receipt) {
          assertReceiptSuccess(lastHash, receipt, rejectOnRevert);
          return { hash: lastHash, receipt: receipt };
        }
        continue;
      }

      throw sendError;
    }
  }

  throw new Error("sendTransactionWithRetry: unreachable");
}
