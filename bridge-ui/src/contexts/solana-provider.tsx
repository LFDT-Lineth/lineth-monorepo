"use client";

import type { PropsWithChildren } from "react";
import { type Adapter, WalletAdapterNetwork, ConnectionProvider, WalletProvider, clusterApiUrl } from "@/lib/solana";
import { DynamicSolanaProvider } from "./dynamic-solana-provider";

const endpoint = clusterApiUrl(WalletAdapterNetwork.Mainnet);

const wallets: Adapter[] = [];

type SolanaWalletProviderProps = PropsWithChildren;

export function SolanaWalletProvider({ children }: SolanaWalletProviderProps) {
  return (
    <ConnectionProvider endpoint={endpoint}>
      <WalletProvider wallets={wallets} autoConnect>
        <DynamicSolanaProvider>{children}</DynamicSolanaProvider>
      </WalletProvider>
    </ConnectionProvider>
  );
}
