"use client";

import { useDynamicContext } from "@dynamic-labs/sdk-react-core";
import Bridge from "../form";
import TransactionHistory from "../transaction-history";
import { useNativeBridgeNavigationStore } from "@/stores";
import BridgeSkeleton from "./skeleton";
import WrongNetwork from "../wrong-network";

export default function BridgeLayout() {
  const isTransactionHistoryOpen = useNativeBridgeNavigationStore.useIsTransactionHistoryOpen();
  const { sdkHasLoaded, primaryWallet } = useDynamicContext();

  if (!sdkHasLoaded) {
    return <BridgeSkeleton />;
  }

  if (primaryWallet && primaryWallet.connector.connectedChain !== "EVM") {
    return <WrongNetwork />;
  }

  return isTransactionHistoryOpen ? <TransactionHistory /> : <Bridge />;
}
