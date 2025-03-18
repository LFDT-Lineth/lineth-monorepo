"use client";

import { useDynamicContext, useIsLoggedIn } from "@/lib/dynamic";
import { useAccount } from "wagmi";
import Bridge from "../form";
import TransactionHistory from "../transaction-history";
import { supportedChainIds } from "@/lib/wagmi";
import { useTokens } from "@/hooks";
import { useChainStore, FormStoreProvider, FormState, useNativeBridgeNavigationStore } from "@/stores";
import { ChainLayer } from "@/types";
import WrongNetwork from "../wrong-network";
import BridgeSkeleton from "./skeleton";

export default function BridgeLayout() {
  const isTransactionHistoryOpen = useNativeBridgeNavigationStore.useIsTransactionHistoryOpen();
  const { chain, address } = useAccount();
  const isLoggedIn = useIsLoggedIn();
  const { sdkHasLoaded } = useDynamicContext();
  const tokens = useTokens();
  const fromChain = useChainStore.useFromChain();

  if (!sdkHasLoaded) {
    return <BridgeSkeleton />;
  }

  if (isLoggedIn && (!chain?.id || !supportedChainIds.includes(chain.id))) {
    return <WrongNetwork />;
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
