import { HistoryActionsForCompleteTxCaching } from "@/stores";
import { BridgeTransaction } from "@/types";

import { isTimestampTooOld } from "./isTimestampTooOld";

/**
 * Save to transaction cache if transaction is not stale.
 */
export function saveToTransactionCache(
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  tx: BridgeTransaction,
): void {
  if (isTimestampTooOld(tx.timestamp)) {
    return;
  }
  historyStoreActions.setCompleteTx(tx);
}
