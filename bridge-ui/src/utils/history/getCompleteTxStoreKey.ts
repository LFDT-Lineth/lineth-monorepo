import { BridgeTransaction, SupportedChainIds } from "@/types";

export function getCompleteTxStoreKeyForTx(transaction: BridgeTransaction): string {
  return getCompleteTxStoreKey(transaction.fromChain.id, transaction.bridgingTx);
}

export function getCompleteTxStoreKey(fromChainId: SupportedChainIds, bridgingTxHash: string): string {
  return `${fromChainId}-${bridgingTxHash}`;
}
