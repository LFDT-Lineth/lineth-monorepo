import { useSendTransaction, useWaitForTransactionReceipt } from "wagmi";

import { type TransactionRequest } from "@/adapters";

export default function useTransactionSender(txArgs: TransactionRequest | undefined) {
  const { data: hash, mutate: sendTransaction, isPending, error, isError, isSuccess } = useSendTransaction();

  const { isLoading: isConfirming, isSuccess: isConfirmed } = useWaitForTransactionReceipt({ hash });

  return {
    send: txArgs ? () => sendTransaction(txArgs) : undefined,
    isPending,
    isConfirming,
    isConfirmed,
    isError,
    error,
    isSuccess,
  };
}
