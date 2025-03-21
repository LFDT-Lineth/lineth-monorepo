import { useSendTransaction, useWaitForTransactionReceipt } from "wagmi";
import useClaimTxArgs from "./transaction-args/useClaimTransactionTxArgs";
import { BridgeTransactionType, CctpV2BridgeMessage, Chain, NativeBridgeMessage, TransactionStatus } from "@/types";

type UseClaimProps = {
  status?: TransactionStatus;
  type?: BridgeTransactionType;
  fromChain?: Chain;
  toChain?: Chain;
  args?: NativeBridgeMessage | CctpV2BridgeMessage;
};

const useClaim = (props: UseClaimProps) => {
  const transactionArgs = useClaimTxArgs(props);
  const { data: hash, sendTransaction, isPending, error, isSuccess } = useSendTransaction();

  const { isLoading: isConfirming, isSuccess: isConfirmed } = useWaitForTransactionReceipt({
    hash,
  });

  return {
    transactionType: "claim",
    claim: transactionArgs ? () => sendTransaction(transactionArgs.args) : undefined,
    isPending: isPending,
    isConfirming,
    isConfirmed,
    error,
    isSuccess,
  };
};

export default useClaim;
