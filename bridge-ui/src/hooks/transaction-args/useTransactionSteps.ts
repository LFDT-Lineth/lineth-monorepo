import { useEffect, useMemo } from "react";

import { QueryObserverResult, RefetchOptions } from "@tanstack/react-query";
import { ReadContractErrorType } from "@wagmi/core";
import { encodeFunctionData, erc20Abi } from "viem";
import { useConnection } from "wagmi";

import { type TransactionRequest, getAdapter } from "@/adapters";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER } from "@/constants/general";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { isEth } from "@/utils/tokens";

import useBridgeFees from "../fees/useBridgeFees";
import useAllowance from "../useAllowance";

export type TransactionArgs =
  | {
      type: string;
      adapterId: string;
      args: TransactionRequest;
      refetchAllowance?: (options?: RefetchOptions) => Promise<QueryObserverResult<bigint, ReadContractErrorType>>;
    }
  | undefined;

export default function useTransactionSteps(): TransactionArgs {
  const { isConnected } = useConnection();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);
  const recipient = useFormStore((state) => state.recipient);
  const selectedMode = useFormStore((state) => state.selectedMode);
  const claim = useFormStore((state) => state.claim);
  const setClaim = useFormStore((state) => state.setClaim);

  const { fees, hasValidFeeData, resolvedClaimType } = useBridgeFees();
  const { allowance, refetchAllowance } = useAllowance();

  useEffect(() => {
    if (resolvedClaimType && resolvedClaimType !== claim) {
      setClaim(resolvedClaimType);
    }
  }, [resolvedClaimType, claim, setClaim]);

  return useMemo((): TransactionArgs => {
    if (amount === null) return;

    const adapter = getAdapter(token, fromChain, toChain);
    if (!adapter) return;
    if (amount > 0n && !hasValidFeeData) return;

    const approvalTarget = adapter.getApprovalTarget(token, fromChain);
    const needsApproval = approvalTarget !== undefined && !isEth(token);

    if (isConnected && needsApproval) {
      if (allowance === undefined) return;

      const adapterPreSteps = adapter.getPreSteps?.({ token, fromChain, amount, allowance });
      if (adapterPreSteps && adapterPreSteps.length > 0) {
        const step = adapterPreSteps[0];
        return { type: step.id, adapterId: adapter.id, args: step.tx, refetchAllowance };
      }

      if (allowance < amount) {
        return {
          type: "approve",
          adapterId: adapter.id,
          args: {
            to: token[fromChain.layer],
            data: encodeFunctionData({
              abi: erc20Abi,
              functionName: "approve",
              args: [approvalTarget, amount],
            }),
            value: 0n,
            chainId: fromChain.id,
          },
          refetchAllowance,
        };
      }
    }

    const toAddress = isConnected ? recipient : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;
    const depositTx = adapter.buildDepositTx({
      token,
      amount,
      recipient: toAddress,
      fromChain,
      toChain,
      fees,
      options: { selectedMode: selectedMode ?? undefined },
    });

    if (!depositTx) return;

    return { type: adapter.id, adapterId: adapter.id, args: depositTx };
  }, [
    isConnected,
    token,
    fromChain,
    toChain,
    amount,
    recipient,
    selectedMode,
    fees,
    allowance,
    refetchAllowance,
    hasValidFeeData,
  ]);
}
