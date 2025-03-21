import { BridgeTransaction } from "@/types";
import { SupportedChainId } from "@/lib/wagmi";

export function getCompleteTxStoreKeyForTx(transaction: BridgeTransaction): string {
  return getCompleteTxStoreKey(transaction.fromChain.id, transaction.bridgingTx);
}

export function getCompleteTxStoreKey(fromChainId: SupportedChainId, bridgingTxHash: string): string {
  return `${fromChainId}-${bridgingTxHash}`;
}
