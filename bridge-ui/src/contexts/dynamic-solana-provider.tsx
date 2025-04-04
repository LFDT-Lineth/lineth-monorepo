"use client";

import { type PropsWithChildren, useCallback, useEffect, useMemo } from "react";
import { type Wallet, useDynamicContext, useDynamicEvents, type SolanaWalletConnector } from "@/lib/dynamic";
import { useWallet } from "@/lib/solana";

const getSolanaConnector = (wallet: Wallet | null): SolanaWalletConnector | undefined => {
  if (wallet?.connector.connectedChain === "SOL") {
    return wallet.connector as SolanaWalletConnector;
  }
};

type DynamicSolanaProviderProps = PropsWithChildren;

export function DynamicSolanaProvider({ children }: DynamicSolanaProviderProps) {
  const { disconnect, select, wallets } = useWallet();

  const { primaryWallet } = useDynamicContext();

  useDynamicEvents("logout", () => {
    disconnect();
  });

  useEffect(() => {
    if (primaryWallet?.connector.connectedChain !== "SOL") {
      disconnect();
    }
  }, [primaryWallet?.connector.connectedChain, disconnect]);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const solanaWallet = useMemo(() => getSolanaConnector(primaryWallet), [primaryWallet?.connector.connectedChain]);

  const handleConnectedSolanaWallet = useCallback(async () => {
    if (!solanaWallet) {
      return;
    }

    const wallet = wallets.find((wallet) => wallet.adapter.name === solanaWallet.name);
    if (wallet) {
      select(wallet.adapter.name);
    }
  }, [solanaWallet, wallets, select]);

  useEffect(() => {
    handleConnectedSolanaWallet();
  }, [handleConnectedSolanaWallet]);

  return children;
}
