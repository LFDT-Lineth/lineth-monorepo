import { Transaction, Txpool } from "./types.js";

export const parsePendingTransactions = (pool: Txpool): Transaction[] => {
  const pendingAddresses = Object.values(pool.pending);
  const transactionsByNonce = pendingAddresses.map((txsByNonce) => Object.values(txsByNonce));
  const transactions = transactionsByNonce.flat();
  return transactions;
};

export const getPendingTransactions = (sourcePool: Transaction[], targetPool: Transaction[]): Transaction[] => {
  const targetPendingTransactions = new Set(targetPool.map((tx) => tx.hash));
  return sourcePool.filter((tx) => !targetPendingTransactions.has(tx.hash));
};
