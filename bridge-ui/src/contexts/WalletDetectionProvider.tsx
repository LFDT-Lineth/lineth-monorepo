"use client";
import { createContext, ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";

interface EIP6963ProviderInfo {
  name: string;
  uuid: string;
  icon: string;
  rdns: string;
}

interface EIP6963ProviderDetail {
  info: EIP6963ProviderInfo;
  provider: unknown;
}

interface EIP6963AnnounceProviderEvent extends Event {
  detail: EIP6963ProviderDetail;
}

interface WalletDetectionContextType {
  walletsInstalled: string[];
}

const WalletDetectionContext = createContext<WalletDetectionContextType>({
  walletsInstalled: [],
});

export const useWalletDetection = () => useContext(WalletDetectionContext);

interface Props {
  children: ReactNode;
}

export default function WalletDetectionProvider({ children }: Props) {
  const [walletsInstalled, setWalletsInstalled] = useState<string[]>([]);
  const wallets = useRef(new Set<string>());
  const isInitialized = useRef(false);

  useEffect(() => {
    // Defer initialization to avoid setState during render
    const initWallets = () => {
      if (isInitialized.current) return;
      isInitialized.current = true;

      // Check for Binance Chain Wallet until it supports EIP-6963
      if (window.BinanceChain) {
        wallets.current.add("BNB");
      }

      const checkWallet = (event: EIP6963AnnounceProviderEvent) => {
        let name = event.detail?.info?.name;

        if (name) {
          // Use less char for GTM event
          name = name.replace(" Wallet", "");

          wallets.current.add(name);

          const arrayOfWallets = Array.from(wallets.current);
          // Use setTimeout to defer state update
          setTimeout(() => {
            setWalletsInstalled(arrayOfWallets);
          }, 0);
        }
      };

      window.addEventListener("eip6963:announceProvider", checkWallet as EventListener);
      window.dispatchEvent(new Event("eip6963:requestProvider"));
    };

    // Defer initialization to next tick
    setTimeout(initWallets, 0);
  }, []);

  const value = useMemo(() => ({ walletsInstalled }), [walletsInstalled]);

  return <WalletDetectionContext.Provider value={value}>{children}</WalletDetectionContext.Provider>;
}
