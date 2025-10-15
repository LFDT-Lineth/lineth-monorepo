import { useSendTransaction, useWaitForTransactionReceipt } from "wagmi";
import useTransactionArgs from "./transaction-args/useTransactionArgs";

const useBridge = () => {
  const transactionArgs = useTransactionArgs();

  const { data: hash, sendTransaction, isPending, error, isError, isSuccess } = useSendTransaction();

  const { isLoading: isConfirming, isSuccess: isConfirmed } = useWaitForTransactionReceipt({
    hash,
  });

  return {
    transactionType: transactionArgs?.type,
    refetchAllowance: transactionArgs?.refetchAllowance,
    bridge: transactionArgs ? () => sendTransaction(transactionArgs.args as any) : undefined,
    isPending: isPending,
    isConfirming,
    isConfirmed,
    isError,
    error,
    isSuccess,
  };
};

export default useBridge;
