"use client";

import { useEffect, useState } from "react";

import { useConnection } from "wagmi";

import { SOLANA_CHAIN } from "@/constants/chains";
import { useNativeBridgeNavigationStore } from "@/stores/nativeBridgeNavigationStore";

import Bridge from "../form";
import TransactionHistory from "../transaction-history";
import BridgeSkeleton from "./skeleton";
import WrongNetwork from "../wrong-network";

export default function BridgeLayout() {
  const isTransactionHistoryOpen = useNativeBridgeNavigationStore.useIsTransactionHistoryOpen();
  const [mounted, setMounted] = useState(false);
  const { address, chainId } = useConnection();

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return <BridgeSkeleton />;
  }

  if (address && chainId === SOLANA_CHAIN) {
    return <WrongNetwork />;
  }

  return isTransactionHistoryOpen ? <TransactionHistory /> : <Bridge />;
}
