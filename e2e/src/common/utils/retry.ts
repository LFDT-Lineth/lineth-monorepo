import { Abi, BaseError, Client, decodeErrorResult, Hash, RawContractError, TransactionReceipt } from "viem";
import {
  waitForTransactionReceipt,
  getTransactionReceipt,
  getTransaction,
  call,
  SendTransactionErrorType,
  WaitForTransactionReceiptErrorType,
} from "viem/actions";

import { createTestLogger } from "../../config/logger";

const DEFAULT_RECEIPT_TIMEOUT_MS = 30_000;
const DEFAULT_FEE_BUMP_FACTOR = 115n;
const DEFAULT_FEE_BUMP_STEP = 20n;
const MAX_FEE_MULTIPLIER = 10n;
const DEFAULT_MAX_RETRIES = 20;
const DEFAULT_OVERALL_TIMEOUT_MS = 3 * 60_000;
const DEFAULT_RETRY_ON_REVERT_DELAY_MS = 1_000;

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
  abi?: Abi;
  retryOnRevert?: boolean;
  retryOnRevertDelayMs?: number;
  beforeRetry?: () => Promise<void>;
};

export type TransactionResult = {
  hash: Hash;
  receipt: TransactionReceipt;
};

export type GasPriceFeeOverrides = {
  gasPrice: bigint;
};

export type SendTransactionWithGasPriceRetryOptions = SendTransactionWithRetryOptions;

type FeeState<TFees> = {
  fees: TFees;
  baseFees: TFees;
};

type FeeBumpResult<TFees> = {
  fees: TFees;
  wasCapped: boolean;
};

type FeeBumpLogContext<TFees> = {
  hash: Hash;
  attempt: number;
  bumpFactor: bigint;
  previousFees: TFees;
  nextFees: TFees;
  baseFees: TFees;
  wasCapped: boolean;
};

type FeeRetryStrategy<TFees> = {
  errorPrefix: string;
  getInitialFeeState: (client: Client, hash: Hash) => Promise<FeeState<TFees>>;
  getNextFeeState: (fees: TFees, baseFees: TFees, bumpFactor: bigint) => FeeBumpResult<TFees>;
  logInitialSend: (hash: Hash, fees: TFees) => void;
  logBump: (context: FeeBumpLogContext<TFees>) => void;
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

function hasErrorNameInChain(error: unknown, errorNames: Set<string>): boolean {
  if (error instanceof BaseError) {
    return (
      error.walk((cause) => {
        const name = (cause as { name?: string }).name;
        return typeof name === "string" && errorNames.has(name);
      }) !== null
    );
  }

  let current = error as { name?: string; cause?: unknown } | undefined;
  let depth = 0;

  while (current && depth < 10) {
    if (current.name && errorNames.has(current.name)) {
      return true;
    }

    if (!current.cause || typeof current.cause !== "object") {
      return false;
    }

    current = current.cause as { name?: string; cause?: unknown };
    depth++;
  }

  return false;
}

export function isContractRevert(error: unknown): boolean {
  return hasErrorNameInChain(error, new Set(["ContractFunctionRevertedError", "CallExecutionError"]));
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
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

function capBumpedFees(fees: FeeOverrides, baseFees: FeeOverrides): FeeOverrides {
  if (
    fees.maxFeePerGas === undefined ||
    fees.maxPriorityFeePerGas === undefined ||
    baseFees.maxFeePerGas === undefined ||
    baseFees.maxPriorityFeePerGas === undefined
  ) {
    throw new Error("sendTransactionWithRetry: non EIP-1559 transaction");
  }

  const maxAllowedFeePerGas = baseFees.maxFeePerGas * MAX_FEE_MULTIPLIER;
  const maxAllowedPriorityFeePerGas = baseFees.maxPriorityFeePerGas * MAX_FEE_MULTIPLIER;

  return {
    maxFeePerGas: fees.maxFeePerGas > maxAllowedFeePerGas ? maxAllowedFeePerGas : fees.maxFeePerGas,
    maxPriorityFeePerGas:
      fees.maxPriorityFeePerGas > maxAllowedPriorityFeePerGas ? maxAllowedPriorityFeePerGas : fees.maxPriorityFeePerGas,
  };
}

function bumpGasPriceFromTx(tx: { gasPrice: bigint | undefined }, bumpFactor: bigint): GasPriceFeeOverrides {
  if (tx.gasPrice === undefined) {
    throw new Error("sendTransactionWithGasPriceRetry: transaction has no gasPrice");
  }

  return { gasPrice: (tx.gasPrice * bumpFactor) / 100n };
}

function capBumpedGasPrice(fees: GasPriceFeeOverrides, baseFees: GasPriceFeeOverrides): GasPriceFeeOverrides {
  const maxAllowedGasPrice = baseFees.gasPrice * MAX_FEE_MULTIPLIER;
  return { gasPrice: fees.gasPrice > maxAllowedGasPrice ? maxAllowedGasPrice : fees.gasPrice };
}

async function getRevertReason(
  client: Client,
  hash: Hash,
  receipt: TransactionReceipt,
  abi?: Abi,
): Promise<string | undefined> {
  try {
    const tx = await getTransaction(client, { hash });
    await call(client, {
      account: tx.from,
      to: tx.to,
      data: tx.input,
      value: tx.value,
      gas: tx.gas,
      blockNumber: receipt.blockNumber,
    });
    return "unknown (eth_call did not revert on replay)";
  } catch (err) {
    if (!(err instanceof BaseError)) return undefined;

    const rawError = err.walk() as RawContractError;
    if (rawError.data && abi) {
      const data = typeof rawError.data === "object" ? rawError.data?.data : rawError.data;
      if (!data) return undefined;

      try {
        const decoded = decodeErrorResult({ abi, data });
        const args = decoded.args?.map((a) => (typeof a === "bigint" ? a.toString() : JSON.stringify(a)));
        return `${decoded.errorName}(${args?.join(", ") ?? ""})`;
      } catch {
        return `raw revert data: ${data}`;
      }
    }

    return (err as Error).message ?? String(err);
  }
}

async function assertReceiptSuccess(
  client: Client,
  hash: Hash,
  receipt: TransactionReceipt,
  rejectOnRevert: boolean,
  abi?: Abi,
): Promise<void> {
  if (rejectOnRevert && receipt.status === "reverted") {
    const reason = await getRevertReason(client, hash, receipt, abi);
    throw new Error(`Transaction reverted: hash=${hash} blockNumber=${receipt.blockNumber}\nRevert reason: ${reason}`);
  }
}

/* ---------------- final wrapper ---------------- */

async function getEip1559FeeState(client: Client, hash: Hash): Promise<FeeState<FeeOverrides>> {
  const tx = await getTransaction(client, { hash });
  const fees = {
    maxPriorityFeePerGas: tx.maxPriorityFeePerGas,
    maxFeePerGas: tx.maxFeePerGas,
  };
  return { fees, baseFees: fees };
}

function getNextEip1559FeeState(
  fees: FeeOverrides,
  baseFees: FeeOverrides,
  bumpFactor: bigint,
): FeeBumpResult<FeeOverrides> {
  const nextFees = bumpFeesFromTx(fees, bumpFactor);
  const cappedFees = capBumpedFees(nextFees, baseFees);
  return {
    fees: cappedFees,
    wasCapped:
      cappedFees.maxFeePerGas !== nextFees.maxFeePerGas ||
      cappedFees.maxPriorityFeePerGas !== nextFees.maxPriorityFeePerGas,
  };
}

function logEip1559InitialSend(hash: Hash, fees: FeeOverrides): void {
  logger.debug(
    `tx sent hash=${hash} maxFeePerGas=${fees.maxFeePerGas} maxPriorityFeePerGas=${fees.maxPriorityFeePerGas}`,
  );
}

function logEip1559Bump(context: FeeBumpLogContext<FeeOverrides>): void {
  const { hash, attempt, bumpFactor, previousFees, nextFees, baseFees, wasCapped } = context;
  logger.debug(
    `bumping fees hash=${hash} attempt=${attempt + 1} bumpFactor=${bumpFactor} ` +
      `prevMaxFeePerGas=${previousFees.maxFeePerGas} nextMaxFeePerGas=${nextFees.maxFeePerGas} ` +
      `prevMaxPriorityFeePerGas=${previousFees.maxPriorityFeePerGas} nextMaxPriorityFeePerGas=${nextFees.maxPriorityFeePerGas}`,
  );

  if (wasCapped) {
    logger.debug(
      `fee cap reached hash=${hash} maxFeePerGasCap=${baseFees.maxFeePerGas! * MAX_FEE_MULTIPLIER} ` +
        `maxPriorityFeePerGasCap=${baseFees.maxPriorityFeePerGas! * MAX_FEE_MULTIPLIER}`,
    );
  }
}

async function getGasPriceFeeState(client: Client, hash: Hash): Promise<FeeState<GasPriceFeeOverrides>> {
  const tx = await getTransaction(client, { hash });
  if (tx.gasPrice === undefined) {
    throw new Error("sendTransactionWithGasPriceRetry: transaction has no gasPrice");
  }

  const fees = { gasPrice: tx.gasPrice };
  return { fees, baseFees: fees };
}

function getNextGasPriceFeeState(
  fees: GasPriceFeeOverrides,
  baseFees: GasPriceFeeOverrides,
  bumpFactor: bigint,
): FeeBumpResult<GasPriceFeeOverrides> {
  const nextFees = bumpGasPriceFromTx(fees, bumpFactor);
  const cappedFees = capBumpedGasPrice(nextFees, baseFees);
  return {
    fees: cappedFees,
    wasCapped: cappedFees.gasPrice !== nextFees.gasPrice,
  };
}

function logGasPriceInitialSend(hash: Hash, fees: GasPriceFeeOverrides): void {
  logger.debug(`tx sent hash=${hash} gasPrice=${fees.gasPrice}`);
}

function logGasPriceBump(context: FeeBumpLogContext<GasPriceFeeOverrides>): void {
  const { hash, attempt, bumpFactor, previousFees, nextFees, baseFees, wasCapped } = context;
  logger.debug(
    `bumping gasPrice hash=${hash} attempt=${attempt + 1} bumpFactor=${bumpFactor} ` +
      `prevGasPrice=${previousFees.gasPrice} nextGasPrice=${nextFees.gasPrice}`,
  );

  if (wasCapped) {
    logger.debug(`gasPrice cap reached hash=${hash} cap=${baseFees.gasPrice * MAX_FEE_MULTIPLIER}`);
  }
}

async function sendTransactionWithFeeRetry<TFees>(
  client: Client,
  sendFn: (fees?: TFees) => Promise<Hash>,
  options: SendTransactionWithRetryOptions = {},
  strategy: FeeRetryStrategy<TFees>,
): Promise<TransactionResult> {
  const receiptTimeoutMs = options.receiptTimeoutMs ?? DEFAULT_RECEIPT_TIMEOUT_MS;
  const feeBumpFactor = options.feeBumpFactor ?? DEFAULT_FEE_BUMP_FACTOR;
  const maxRetries = options.maxRetries ?? DEFAULT_MAX_RETRIES;
  const overallTimeoutMs = options.overallTimeoutMs ?? DEFAULT_OVERALL_TIMEOUT_MS;
  const rejectOnRevert = options.rejectOnRevert ?? true;
  const abi = options.abi;
  const retryOnRevert = options.retryOnRevert ?? false;
  const retryOnRevertDelayMs = options.retryOnRevertDelayMs ?? DEFAULT_RETRY_ON_REVERT_DELAY_MS;
  const beforeRetry = options.beforeRetry;

  const startedAt = Date.now();
  let lastHash!: Hash;
  let fees!: TFees;
  let baseFees!: TFees;
  let attempt = 0;
  let needsSend = true;

  function withinLimits(): boolean {
    return attempt < maxRetries && Date.now() - startedAt <= overallTimeoutMs;
  }

  async function freshSend(): Promise<void> {
    lastHash = await sendFn();
    const feeState = await strategy.getInitialFeeState(client, lastHash);
    fees = feeState.fees;
    baseFees = feeState.baseFees;
    strategy.logInitialSend(lastHash, fees);
  }

  async function handleRevertRetry(hash: Hash, receipt: TransactionReceipt): Promise<boolean> {
    if (receipt.status !== "reverted" || !retryOnRevert || !withinLimits()) return false;

    const reason = await getRevertReason(client, hash, receipt, abi);
    logger.debug(`tx reverted, will retry: hash=${hash} attempt=${attempt} reason=${reason}`);

    await beforeRetry?.();
    await sleep(retryOnRevertDelayMs);
    needsSend = true;
    attempt++;
    return true;
  }

  async function retryAfterSimulationRevert(error: unknown): Promise<boolean> {
    if (!(retryOnRevert && isContractRevert(error) && withinLimits())) {
      return false;
    }

    logger.debug(`sendFn reverted at simulation, retrying: attempt=${attempt} error=${(error as Error).message}`);
    await beforeRetry?.();
    await sleep(retryOnRevertDelayMs);
    needsSend = true;
    attempt++;
    return true;
  }

  while (attempt <= maxRetries) {
    /* ---------- (re)send ---------- */
    if (needsSend) {
      needsSend = false;
      try {
        await freshSend();
      } catch (err) {
        if (await retryAfterSimulationRevert(err)) {
          continue;
        }
        throw err;
      }
    }

    /* ---------- hard deadline ---------- */
    if (Date.now() - startedAt > overallTimeoutMs) {
      logger.debug(`overall timeout exceeded hash=${lastHash} attempt=${attempt}; probing receipt`);

      const txReceipt = await safeGetReceipt(client, lastHash);
      if (txReceipt) {
        logger.debug(`tx confirmed during final probe hash=${lastHash} blockNumber=${txReceipt.blockNumber}`);
        await assertReceiptSuccess(client, lastHash, txReceipt, rejectOnRevert, abi);
        return { hash: lastHash, receipt: txReceipt };
      }

      throw new Error(`${strategy.errorPrefix}: overall timeout exceeded`);
    }

    /* ---------- primary wait ---------- */
    try {
      logger.debug(`waiting for receipt hash=${lastHash} attempt=${attempt} timeoutMs=${receiptTimeoutMs}`);

      const receipt = await waitForTransactionReceipt(client, {
        hash: lastHash,
        timeout: receiptTimeoutMs,
      });

      logger.debug(`tx confirmed hash=${lastHash} blockNumber=${receipt.blockNumber} status=${receipt.status}`);

      if (await handleRevertRetry(lastHash, receipt)) continue;
      await assertReceiptSuccess(client, lastHash, receipt, rejectOnRevert, abi);
      return { hash: lastHash, receipt };
    } catch (err) {
      if (!isReceiptTimeout(err)) throw err;

      logger.debug(`receipt timeout for hash=${lastHash} attempt=${attempt}`);
    }

    /* ---------- race guard ---------- */
    const raceConditionReceipt = await safeGetReceipt(client, lastHash);
    if (raceConditionReceipt) {
      logger.debug(`tx mined during timeout race hash=${lastHash} blockNumber=${raceConditionReceipt.blockNumber}`);

      if (await handleRevertRetry(lastHash, raceConditionReceipt)) continue;
      await assertReceiptSuccess(client, lastHash, raceConditionReceipt, rejectOnRevert, abi);
      return { hash: lastHash, receipt: raceConditionReceipt };
    }

    if (attempt === maxRetries) {
      logger.debug(`max retries reached hash=${lastHash} attempt=${attempt}; probing receipt`);

      const receipt = await safeGetReceipt(client, lastHash);
      if (receipt) {
        await assertReceiptSuccess(client, lastHash, receipt, rejectOnRevert, abi);
        return { hash: lastHash, receipt: receipt };
      }

      throw new Error(`${strategy.errorPrefix}: max retries exceeded`);
    }

    /* ---------- bump from actual tx ---------- */
    const bumpFactor = feeBumpFactor + BigInt(attempt) * DEFAULT_FEE_BUMP_STEP;
    const previousFees = fees;
    const nextFeeState = strategy.getNextFeeState(previousFees, baseFees, bumpFactor);
    strategy.logBump({
      hash: lastHash,
      attempt,
      bumpFactor,
      previousFees,
      nextFees: nextFeeState.fees,
      baseFees,
      wasCapped: nextFeeState.wasCapped,
    });

    fees = nextFeeState.fees;
    attempt++;

    try {
      lastHash = await sendFn(fees);

      logger.debug(`replacement tx sent hash=${lastHash} attempt=${attempt}`);
    } catch (sendError) {
      if (isNonceTooLow(sendError)) {
        logger.debug(`nonce too low while retrying hash=${lastHash}; original tx likely mined`);

        const receipt = await safeGetReceipt(client, lastHash);
        if (receipt) {
          if (await handleRevertRetry(lastHash, receipt)) continue;
          await assertReceiptSuccess(client, lastHash, receipt, rejectOnRevert, abi);
          return { hash: lastHash, receipt: receipt };
        }
        continue;
      }

      if (await retryAfterSimulationRevert(sendError)) {
        continue;
      }

      throw sendError;
    }
  }

  throw new Error(`${strategy.errorPrefix}: unreachable`);
}

export async function sendTransactionWithRetry(
  client: Client,
  sendFn: (fees?: FeeOverrides) => Promise<Hash>,
  options: SendTransactionWithRetryOptions = {},
): Promise<TransactionResult> {
  return sendTransactionWithFeeRetry(client, sendFn, options, {
    errorPrefix: "sendTransactionWithRetry",
    getInitialFeeState: getEip1559FeeState,
    getNextFeeState: getNextEip1559FeeState,
    logInitialSend: logEip1559InitialSend,
    logBump: logEip1559Bump,
  });
}

export async function sendTransactionWithGasPriceRetry(
  client: Client,
  sendFn: (fees?: GasPriceFeeOverrides) => Promise<Hash>,
  options: SendTransactionWithGasPriceRetryOptions = {},
): Promise<TransactionResult> {
  return sendTransactionWithFeeRetry(client, sendFn, options, {
    errorPrefix: "sendTransactionWithGasPriceRetry",
    getInitialFeeState: getGasPriceFeeState,
    getNextFeeState: getNextGasPriceFeeState,
    logInitialSend: logGasPriceInitialSend,
    logBump: logGasPriceBump,
  });
}
