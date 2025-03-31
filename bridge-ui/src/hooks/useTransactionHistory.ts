import { useAccount } from "wagmi";
import { useQuery } from "@tanstack/react-query";
import { HistoryActionsForCompleteTxCaching, useChainStore } from "@/stores";
import useLineaSDK from "./useLineaSDK";
import useTokens from "./useTokens";
import { useHistoryStore } from "@/stores";
import { fetchTransactionsHistory } from "@/utils";

const useTransactionHistory = () => {
  const { address } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const { lineaSDK } = useLineaSDK();
  const tokens = useTokens();
  const { setCompleteTx, getCompleteTx } = useHistoryStore((state) => ({
    setCompleteTx: state.setCompleteTx,
    getCompleteTx: state.getCompleteTx,
  }));
  const historyStoreActions: HistoryActionsForCompleteTxCaching = {
    setCompleteTx,
    getCompleteTx,
  };

  const { data, isLoading, refetch } = useQuery({
    enabled: !!address && !!fromChain && !!toChain,
    queryKey: ["transactionHistory", address, fromChain.id, toChain.id],
    queryFn: () =>
      fetchTransactionsHistory({
        lineaSDK,
        fromChain,
        toChain,
        address: address!,
        tokens,
        historyStoreActions,
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
