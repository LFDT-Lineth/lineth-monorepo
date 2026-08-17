import {
  type AccessList,
  type Hash,
  type Hex,
  isHex,
  keccak256,
  type Signature,
  serializeTransaction,
  type TransactionSerializable,
} from "viem";

import { type ClientType, type TransactionPool } from "./client.js";
import { type Quantity, type Transaction, type Txpool } from "./types.js";

export const parsePendingTransactions = (pool: Txpool): Transaction[] => {
  const pendingAddresses = Object.values(pool.pending);
  const transactionsByNonce = pendingAddresses.map((txsByNonce) => Object.values(txsByNonce));
  const transactions = transactionsByNonce.flat();
  return transactions;
};

export const getTransactionsFromPool = (clientType: ClientType, pool: TransactionPool): Transaction[] => {
  if (clientType === "geth") {
    return parsePendingTransactions(pool as Txpool);
  }

  if (!Array.isArray(pool)) {
    throw new Error("Invalid Besu txpool response: expected an array of pending transactions");
  }

  return pool;
};

export const hasPendingTransactions = (clientType: ClientType, pool: TransactionPool): boolean => {
  if (clientType === "geth") {
    return Object.keys((pool as Txpool).pending).length > 0;
  }

  return Array.isArray(pool) && pool.length > 0;
};

export const getPendingTransactions = (sourcePool: Transaction[], targetPool: Transaction[]): Transaction[] => {
  const targetPendingTransactions = new Set(targetPool.map((tx) => tx.hash));
  return sourcePool.filter((tx) => !targetPendingTransactions.has(tx.hash));
};

export const parseTransactionsFileContent = (content: string, filePath: string): Transaction[] => {
  let transactions: unknown;
  try {
    transactions = JSON.parse(content);
  } catch (error) {
    throw new Error(`Invalid txs file ${filePath}: ${(error as Error).message}`);
  }

  if (!Array.isArray(transactions)) {
    throw new Error(`Invalid txs file ${filePath}: expected a JSON array of transactions`);
  }

  return transactions as Transaction[];
};

const toBigInt = (value: Quantity | undefined, field: string, hash: Hash): bigint => {
  if (value === undefined) {
    throw new Error(`Missing required transaction field ${field} for ${hash}`);
  }

  try {
    return BigInt(value);
  } catch {
    throw new Error(`Invalid transaction field ${field} for ${hash}: expected a numeric quantity`);
  }
};

const toSafeNumber = (value: Quantity | undefined, field: string, hash: Hash): number => {
  const parsed = toBigInt(value, field, hash);
  if (parsed < 0n || parsed > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new Error(`Invalid transaction field ${field} for ${hash}: quantity is outside the safe integer range`);
  }

  return Number(parsed);
};

const toTransactionType = (type: Transaction["type"], hash: Hash): 0 | 1 | 2 => {
  if (typeof type === "string") {
    const normalizedType = type.toLowerCase();
    if (normalizedType === "legacy") {
      return 0;
    }
    if (normalizedType === "eip2930") {
      return 1;
    }
    if (normalizedType === "eip1559") {
      return 2;
    }
  }

  const parsedType = toSafeNumber(type, "type", hash);
  if (parsedType === 0 || parsedType === 1 || parsedType === 2) {
    return parsedType;
  }

  throw new Error(`Unsupported transaction type ${type.toString()} for ${hash}`);
};

const toHexField = (value: Hex | undefined, field: string, hash: Hash): Hex => {
  if (value === undefined) {
    throw new Error(`Missing required transaction field ${field} for ${hash}`);
  }
  if (!isHex(value)) {
    throw new Error(`Invalid transaction field ${field} for ${hash}: expected a hex string`);
  }

  return value;
};

const toOptionalHexField = (value: Hex | undefined, field: string, hash: Hash): Hex | undefined => {
  if (value === undefined) {
    return undefined;
  }

  return toHexField(value, field, hash);
};

const toYParity = (value: Quantity | undefined, hash: Hash): number => {
  const parsed = toBigInt(value, "yParity", hash);
  if (parsed !== 0n && parsed !== 1n) {
    throw new Error(`Invalid transaction field yParity for ${hash}: expected 0 or 1`);
  }

  return Number(parsed);
};

const deriveYParityFromV = (value: Quantity | undefined, hash: Hash): number => {
  const parsed = toBigInt(value, "v", hash);
  if (parsed === 0n || parsed === 1n) {
    return Number(parsed);
  }
  if (parsed === 27n || parsed === 28n) {
    return Number(parsed - 27n);
  }
  if (parsed >= 35n) {
    return Number((parsed - 35n) % 2n);
  }

  throw new Error(`Invalid transaction field v for ${hash}: cannot derive yParity`);
};

const normalizeLegacyV = (value: Quantity | undefined, hash: Hash): bigint => {
  const parsed = toBigInt(value, "v", hash);
  if (parsed === 0n || parsed === 1n) {
    return parsed + 27n;
  }
  if (parsed === 27n || parsed === 28n || parsed >= 35n) {
    return parsed;
  }

  throw new Error(`Invalid transaction field v for ${hash}: expected a valid legacy v value`);
};

const getLegacySignature = (tx: Transaction): Signature => ({
  r: toHexField(tx.r, "r", tx.hash),
  s: toHexField(tx.s, "s", tx.hash),
  v: normalizeLegacyV(tx.v, tx.hash),
});

const getTypedSignature = (tx: Transaction): Signature => ({
  r: toHexField(tx.r, "r", tx.hash),
  s: toHexField(tx.s, "s", tx.hash),
  yParity: tx.yParity !== undefined ? toYParity(tx.yParity, tx.hash) : deriveYParityFromV(tx.v, tx.hash),
});

const getAccessList = (tx: Transaction): AccessList | undefined => tx.accessList ?? undefined;

const getBaseTransaction = (tx: Transaction) => ({
  to: tx.to ?? null,
  nonce: toSafeNumber(tx.nonce, "nonce", tx.hash),
  gas: toBigInt(tx.gas, "gas", tx.hash),
  data: toOptionalHexField(tx.input, "input", tx.hash) ?? "0x",
  value: toBigInt(tx.value, "value", tx.hash),
});

export const toSerializableTransaction = (tx: Transaction): [TransactionSerializable, Signature] => {
  const type = toTransactionType(tx.type, tx.hash);
  const baseTransaction = getBaseTransaction(tx);

  if (type === 0) {
    return [
      {
        ...baseTransaction,
        type: "legacy",
        gasPrice: toBigInt(tx.gasPrice, "gasPrice", tx.hash),
        ...(tx.chainId !== undefined ? { chainId: toSafeNumber(tx.chainId, "chainId", tx.hash) } : {}),
      },
      getLegacySignature(tx),
    ];
  }

  if (type === 1) {
    return [
      {
        ...baseTransaction,
        type: "eip2930",
        chainId: toSafeNumber(tx.chainId, "chainId", tx.hash),
        gasPrice: toBigInt(tx.gasPrice, "gasPrice", tx.hash),
        accessList: getAccessList(tx),
      },
      getTypedSignature(tx),
    ];
  }

  return [
    {
      ...baseTransaction,
      type: "eip1559",
      chainId: toSafeNumber(tx.chainId, "chainId", tx.hash),
      maxFeePerGas: toBigInt(tx.maxFeePerGas, "maxFeePerGas", tx.hash),
      maxPriorityFeePerGas: toBigInt(tx.maxPriorityFeePerGas, "maxPriorityFeePerGas", tx.hash),
      accessList: getAccessList(tx),
    },
    getTypedSignature(tx),
  ];
};

export const serializeVerifiedTxpoolTransaction = (tx: Transaction): Hex => {
  const [transaction, signature] = toSerializableTransaction(tx);
  const rawTransaction = serializeTransaction(transaction, signature);
  const serializedHash = keccak256(rawTransaction);
  if (serializedHash !== tx.hash) {
    throw new Error(`Serialized transaction hash mismatch for ${tx.hash}: got ${serializedHash}`);
  }

  return rawTransaction;
};
