"use client";

import Bridge from "../form";
import TransactionHistory from "../transaction-history";
import { useNativeBridgeNavigationStore } from "@/stores";
import BridgeSkeleton from "./skeleton";
import WrongNetwork from "../wrong-network";
import { useEffect, useState } from "react";
import { useAccount } from "wagmi";
import { SOLANA_CHAIN } from "@/constants";

export default function BridgeLayout() {
  const isTransactionHistoryOpen = useNativeBridgeNavigationStore.useIsTransactionHistoryOpen();
  const [mounted, setMounted] = useState(false);
  const { address, chainId } = useAccount();

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
