"use client";

import { DynamicWagmiConnector, EthereumWalletConnectors, DynamicContextProvider } from "@/lib/dynamic";
import { ReactNode } from "react";
import { config } from "@/lib/wagmi";
import { WagmiProvider } from "wagmi";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const queryClient = new QueryClient();

type Web3ProviderProps = {
  children: ReactNode;
};

export const cssOverrides = `
  .connect-button {
    font-size: 0.875rem;
    border-radius: 1.875rem;
    padding: 0.75rem 1.5rem;
    text-align: center;
    line-height: 1;
    cursor: pointer;
  }

  .connect-button .typography {
    font-size: 0.875rem;
  }

  .dynamic-widget-inline-controls {
    background-color: transparent;
    border: 1px solid white;
    border-radius: 1.875rem;
  }

  .dynamic-widget-inline-controls .network-switch-control__network-name {
    color: white;

    @media screen and (max-width: 912px) {
      display: none;
    }
  }

  .dynamic-widget-inline-controls .network-switch-control__container--error {
    border-radius: 1.875rem 0 0 1.875rem;
  }

  .dynamic-widget-inline-controls .network-switch-control__arrow-icon {
    color: white;
  }

  .account-control__name {
    color: white;

    @media screen and (max-width: 912px) {
      display: none;
    }
  }

  .account-control__icon {
    color: white;
  }
`;

export function Web3Provider({ children }: Web3ProviderProps) {
  return (
    <DynamicContextProvider
      settings={{
        environmentId: process.env.NEXT_PUBLIC_DYNAMIC_ENVIRONMENT_ID!,
        walletConnectors: [EthereumWalletConnectors],
        mobileExperience: "redirect",
        appName: "Linea Bridge",
        cssOverrides,
      }}
    >
      <WagmiProvider config={config}>
        <QueryClientProvider client={queryClient}>
          <DynamicWagmiConnector>{children}</DynamicWagmiConnector>
        </QueryClientProvider>
      </WagmiProvider>
    </DynamicContextProvider>
  );
}
