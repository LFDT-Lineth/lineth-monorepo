"use client";

import { useDynamicContext } from "@/lib/dynamic";
import Bridge from "../form";
import TransactionHistory from "../transaction-history";
import { useNativeBridgeNavigationStore } from "@/stores";
import BridgeSkeleton from "./skeleton";

export default function BridgeLayout() {
  const isTransactionHistoryOpen = useNativeBridgeNavigationStore.useIsTransactionHistoryOpen();
  const { sdkHasLoaded } = useDynamicContext();

  if (!sdkHasLoaded) {
    return <BridgeSkeleton />;
  }

  return isTransactionHistoryOpen ? <TransactionHistory /> : <Bridge />;
}
