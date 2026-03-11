import { useMemo } from "react";

import { useConnection } from "wagmi";

import { type TransactionRequest, getAdapterById } from "@/adapters";
import { BridgeMessage, Chain, TransactionStatus } from "@/types";
import { isUndefined, isUndefinedOrEmptyString } from "@/utils/misc";

import useTransactionSender from "./useTransactionSender";

type UseClaimProps = {
  status?: TransactionStatus;
  adapterId?: string;
  fromChain?: Chain;
  toChain?: Chain;
  args?: BridgeMessage;
};

const useClaim = ({ status, adapterId, fromChain, toChain, args }: UseClaimProps) => {
  const { address } = useConnection();

  const claimTx = useMemo((): TransactionRequest | undefined => {
    if (
      isUndefinedOrEmptyString(address) ||
      isUndefined(status) ||
      isUndefined(adapterId) ||
      isUndefined(fromChain) ||
      isUndefined(toChain) ||
      isUndefined(args)
    )
      return;

    if (status !== TransactionStatus.READY_TO_CLAIM) return;

    const adapter = getAdapterById(adapterId);
    if (!adapter) return;

    return adapter.buildClaimTx?.({ message: args, fromChain, toChain }) ?? undefined;
  }, [address, status, adapterId, fromChain, toChain, args]);

  const { send, ...txState } = useTransactionSender(claimTx);

  return {
    transactionType: "claim" as const,
    claim: send,
    ...txState,
  };
};

export default useClaim;
