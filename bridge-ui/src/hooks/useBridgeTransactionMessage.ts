import { useQuery } from "@tanstack/react-query";
import { useConfig } from "wagmi";

import { getAdapterById } from "@/adapters";
import { BridgeMessage, BridgeTransaction, TransactionStatus } from "@/types";
import { isUndefined } from "@/utils/misc";

const useBridgeTransactionMessage = (
  transaction: BridgeTransaction | undefined,
): { message: BridgeMessage | undefined; isLoading: boolean } => {
  const wagmiConfig = useConfig();
  const adapter = transaction ? getAdapterById(transaction.adapterId) : undefined;

  const { data, isLoading } = useQuery({
    queryKey: ["useBridgeTransactionMessage", transaction?.bridgingTx, transaction?.toChain?.id, transaction?.status],
    queryFn: async () => {
      const { status, fromChain, toChain, message } = transaction as BridgeTransaction;
      if (isUndefined(status) || isUndefined(fromChain) || isUndefined(toChain) || isUndefined(message)) {
        return message;
      }
      if (status !== TransactionStatus.READY_TO_CLAIM) return message;

      await adapter?.prepareClaimMessage?.({ message, fromChain, toChain, wagmiConfig });
      return message;
    },
    enabled: !!transaction,
  });

  return { message: data, isLoading };
};

export default useBridgeTransactionMessage;
