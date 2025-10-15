import { useAccount, useConfig } from "wagmi";
import { useQuery } from "@tanstack/react-query";
import { HistoryActionsForCompleteTxCaching, useChainStore, useHistoryStore } from "@/stores";
import useTokens from "./useTokens";
import { fetchTransactionsHistory } from "@/utils";

const useTransactionHistory = () => {
  const { address } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const tokens = useTokens();
  const wagmiConfig = useConfig();

  const { setCompleteTx, getCompleteTx, deleteCompleteTx } = useHistoryStore((state) => ({
    setCompleteTx: state.setCompleteTx,
    getCompleteTx: state.getCompleteTx,
    deleteCompleteTx: state.deleteCompleteTx,
  }));

  const historyStoreActions: HistoryActionsForCompleteTxCaching = {
    setCompleteTx,
    getCompleteTx,
    deleteCompleteTx,
  };

  const { data, isLoading, refetch } = useQuery({
    enabled: !!address && !!fromChain && !!toChain,
    queryKey: ["transactionHistory", address, fromChain.id, toChain.id],
    queryFn: () =>
      fetchTransactionsHistory({
        fromChain,
        toChain,
        address: address!,
        tokens,
        historyStoreActions,
        wagmiConfig,
      }),
    staleTime: 1000 * 60 * 0.5,
  });

  return {
    transactions: data || [],
    isLoading,
    refetch,
  };
};

export default useTransactionHistory;
