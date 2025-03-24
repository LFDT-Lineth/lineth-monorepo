"use client";

import { useDynamicContext } from "@/lib/dynamic";
import { useAccount } from "wagmi";
import Bridge from "../form";
import TransactionHistory from "../transaction-history";
import { useTokens } from "@/hooks";
import { useChainStore, FormStoreProvider, FormState, useNativeBridgeNavigationStore } from "@/stores";
import { ChainLayer } from "@/types";
import BridgeSkeleton from "./skeleton";

export default function BridgeLayout() {
  const isTransactionHistoryOpen = useNativeBridgeNavigationStore.useIsTransactionHistoryOpen();
  const { address } = useAccount();
  const { sdkHasLoaded } = useDynamicContext();
  const tokens = useTokens();
  const fromChain = useChainStore.useFromChain();

  if (!sdkHasLoaded) {
    return <BridgeSkeleton />;
  }

  if (isTransactionHistoryOpen) {
    return <TransactionHistory />;
  }

  const initialFormState: FormState = {
    token: tokens[0],
    claim: fromChain?.layer === ChainLayer.L1 ? "auto" : "manual",
    amount: null,
    minimumFees: 0n,
    gasFees: 0n,
    bridgingFees: 0n,
    balance: 0n,
    recipient: address || "0x",
  };

  return (
    <FormStoreProvider initialState={initialFormState}>
      <Bridge />
    </FormStoreProvider>
  );
}
