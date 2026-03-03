import { useQuery } from "@tanstack/react-query";
import { useConfig } from "wagmi";

import { getAdapterById } from "@/adapters";
import { BridgeTransaction, TransactionStatus } from "@/types";
import { isUndefined } from "@/utils/misc";

const useClaimingTx = (transaction: BridgeTransaction | undefined): string | undefined => {
  const wagmiConfig = useConfig();
  const adapter = transaction ? getAdapterById(transaction.adapterId) : undefined;

  const { data } = useQuery({
    queryKey: ["useClaimingTx", transaction?.bridgingTx, transaction?.toChain?.id, transaction?.status],
    queryFn: async () => {
      if (
        isUndefined(transaction) ||
        transaction.claimingTx ||
        transaction.status !== TransactionStatus.COMPLETED ||
        !adapter?.getClaimingTxHash
      ) {
        return null;
      }
      return adapter.getClaimingTxHash(transaction.message, transaction.toChain, wagmiConfig);
    },
    enabled:
      !!transaction &&
      !transaction.claimingTx &&
      transaction.status === TransactionStatus.COMPLETED &&
      !!adapter?.getClaimingTxHash,
  });

  return data ?? undefined;
};

export default useClaimingTx;
