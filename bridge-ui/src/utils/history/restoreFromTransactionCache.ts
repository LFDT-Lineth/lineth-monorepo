import { HistoryActionsForCompleteTxCaching } from "@/stores";
import { BridgeTransaction, SupportedChainIds } from "@/types";

import { getCompleteTxStoreKey } from "./getCompleteTxStoreKey";
import { isTimestampTooOld } from "./isTimestampTooOld";

/**
 * Handles transaction cache lookup, validation, and management.
 * Returns true if transaction was found in cache and added to map (skip further processing).
 * Returns false if no valid cache entry exists (continue with processing).
 */
export function restoreFromTransactionCache(
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  chainId: SupportedChainIds,
  transactionHash: string,
  transactionsMap: Map<string, BridgeTransaction>,
  mapKey: string,
): boolean {
  const cacheKey = getCompleteTxStoreKey(chainId, transactionHash);
  const cachedCompletedTx = historyStoreActions.getCompleteTx(cacheKey);

  if (cachedCompletedTx) {
    if (isTimestampTooOld(cachedCompletedTx.timestamp)) {
      historyStoreActions.deleteCompleteTx(cacheKey);
    }
    transactionsMap.set(mapKey, cachedCompletedTx);
    return true; // Found valid cached transaction, skip processing
  }

  return false; // No cache hit, continue processing
}
