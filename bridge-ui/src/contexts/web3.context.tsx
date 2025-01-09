"use client";

import { ReactNode } from "react";
import { WagmiProvider } from "wagmi";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { config, wagmiConfig } from "@/config";
import { createAppKit } from "@reown/appkit/react";
import { chains, wagmiAdapter } from "@/config/wagmi";

const queryClient = new QueryClient();

if (!config.walletConnectId) throw new Error("Project ID is not defined");

const metadata = {
  name: "Linea Bridge",
  description: `The Linea Bridge is a bridge solution, providing secure and efficient cross-chain transactions between Ethereum Layer 1 and Linea networks.
  Discover the future of blockchain interaction with Linea Bridge.`,
  url: "https://bridge.linea.build",
  icons: [],
};

createAppKit({
  adapters: [wagmiAdapter],
  networks: chains,
  projectId: config.walletConnectId,
  metadata,
  features: {
    analytics: true,
    email: false,
    socials: false,
    swaps: false,
    onramp: false,
    history: false,
  },
  enableEIP6963: true,
  coinbasePreference: "eoaOnly",
});

type Web3ProviderProps = {
  children: ReactNode;
};

export function Web3Provider({ children }: Web3ProviderProps) {
  return (
    <WagmiProvider config={wagmiConfig}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </WagmiProvider>
  );
}
