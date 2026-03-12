import { useQuery } from "@tanstack/react-query";
import { useConnection, useConfig, useEstimateGas } from "wagmi";

import { type ClaimContext, type TransactionRequest, getAdapterById } from "@/adapters";
import { BridgeTransaction, TransactionStatus } from "@/types";
import { isUndefinedOrEmptyString } from "@/utils/misc";

import useTransactionSender from "./useTransactionSender";

type UseClaimProps = {
  transaction: BridgeTransaction;
};

const useClaim = ({ transaction }: UseClaimProps) => {
  const { address } = useConnection();
  const wagmiConfig = useConfig();

  const adapter = getAdapterById(transaction.adapterId);
  const hasClaimContext = !!adapter?.getClaimContext;
  const isReadyToClaim = transaction.status === TransactionStatus.READY_TO_CLAIM;

  const { data: claimContext, isLoading: isContextLoading } = useQuery<ClaimContext | undefined>({
    queryKey: ["claimContext", transaction.adapterId, transaction.bridgingTx, address],
    queryFn: () => adapter!.getClaimContext!({ transaction, connectedAddress: address, wagmiConfig }),
    enabled: hasClaimContext && isReadyToClaim,
    staleTime: 30_000,
  });

  const claimOptions = claimContext?.claimOptions;
  const contextResolved = !hasClaimContext || !isContextLoading;

  const { data: claimTx, isLoading: isClaimTxLoading } = useQuery<TransactionRequest | undefined>({
    queryKey: ["claimTx", transaction.adapterId, transaction.bridgingTx, address, claimOptions],
    queryFn: () =>
      adapter!.buildClaimTx!({
        message: transaction.message,
        fromChain: transaction.fromChain,
        toChain: transaction.toChain,
        options: claimOptions,
        wagmiConfig,
      }),
    enabled: !!adapter?.buildClaimTx && !isUndefinedOrEmptyString(address) && isReadyToClaim && contextResolved,
    staleTime: 60_000,
  });

  const shouldSimulate = !!claimOptions && !!claimTx;

  const {
    isLoading: isSimulating,
    isError: simulationFailed,
    isSuccess: simulationPassed,
  } = useEstimateGas({
    to: claimTx?.to,
    data: claimTx?.data,
    value: claimTx?.value,
    chainId: claimTx?.chainId,
    account: address,
    query: {
      enabled: shouldSimulate && !!address,
      retry: 2,
      staleTime: 30_000,
    },
  });

  const effectiveTx = shouldSimulate && !simulationPassed ? undefined : claimTx;
  const { send, ...txState } = useTransactionSender(effectiveTx);

  return {
    transactionType: "claim" as const,
    claim: send,
    claimContext,
    isClaimTxLoading: isClaimTxLoading || (hasClaimContext && isContextLoading),
    isSimulating: shouldSimulate && isSimulating,
    simulationFailed: shouldSimulate && simulationFailed,
    ...txState,
  };
};

export default useClaim;
