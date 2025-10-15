"use client";

import { ReactNode, useEffect, useMemo, useState } from "react";
import { createConfig, http, useReconnect, WagmiProvider } from "wagmi";
import { useWalletDetection } from "./WalletDetectionProvider";
import { useWeb3Auth, Web3AuthContextConfig, Web3AuthProvider } from "@web3auth/modal/react";
import { WEB3AUTH_NETWORK } from "@web3auth/modal";
import { ADAPTER_EVENTS } from "@web3auth/base";
import useGTM from "@/hooks/useGtm";
import { useCachedIdentityToken } from "@/hooks/useCachedIdentityToken";
import { isProd } from "../../next.config.mjs";
import { web3auth } from "@/lib/web3AuthConnector";
import { CHAINS, CHAINS_IDS, CHAINS_RPC_URLS, E2E_TEST_CHAINS, localL1Network, localL2Network } from "@/constants";
import { config as appConfig } from "@/config";

interface DynamicProviderProps {
  children: ReactNode;
}

const clientId = appConfig.web3AuthClientId;

const web3AuthContextConfig: Web3AuthContextConfig = {
  web3AuthOptions: {
    clientId,
    web3AuthNetwork: isProd ? WEB3AUTH_NETWORK.SAPPHIRE_MAINNET : WEB3AUTH_NETWORK.SAPPHIRE_DEVNET,
    defaultChainId: appConfig.e2eTestMode ? "0x1E2EAAC" : "0xe708", // L2 local chain or Linea Mainnet
    uiConfig: {
      appUrl: "https://linea.build/hub/bridge",
      displayErrorsOnModal: true,
    },
    modalConfig: {
      connectors: {
        auth: {
          showOnModal: false,
          label: "Web3Auth",
        },
      },
    },
  },
};

function Web3AuthEventBridge() {
  const { web3Auth } = useWeb3Auth();
  const { walletsInstalled } = useWalletDetection();
  const { trackEvent } = useGTM();
  const { clearTokenCache } = useCachedIdentityToken();

  useEffect(() => {
    if (walletsInstalled?.length > 0) {
      trackEvent({
        event: "wallets_installed",
        wallets: walletsInstalled?.join(","),
      });
    }
  }, [trackEvent, walletsInstalled]);

  useEffect(() => {
    if (!web3Auth || !trackEvent) return;

    const onConnecting = () => {
      trackEvent({
        event: "before_all_clicks",
        click_text_before: "Connect wallet",
        gtm_human_readable_id_before: "gtm-connect-wallet-button",
      });
    };

    const onConnected = (args: { connector: string }) => {
      trackEvent({
        event: "wallet_connected",
        wallet_connected: args?.connector,
        wallets_installed: walletsInstalled?.join(","),
      });
    };

    const onDisconnected = () => {
      clearTokenCache();
      trackEvent({
        event: "before_all_clicks",
        click_text_before: "Connect wallet",
        gtm_human_readable_id_before: "gtm-connect-wallet-button",
      });
    };

    const onErrored = () => {
      clearTokenCache();
      trackEvent({
        event: "before_all_clicks",
        click_text_before: "Connect wallet",
        gtm_human_readable_id_before: "gtm-connect-wallet-button",
      });
    };

    web3Auth.on(ADAPTER_EVENTS.CONNECTING, onConnecting);
    web3Auth.on(ADAPTER_EVENTS.CONNECTED, onConnected);
    web3Auth.on(ADAPTER_EVENTS.DISCONNECTED, onDisconnected);
    web3Auth.on(ADAPTER_EVENTS.ERRORED, onErrored);

    return () => {
      web3Auth.off(ADAPTER_EVENTS.CONNECTING, onConnecting);
      web3Auth.off(ADAPTER_EVENTS.CONNECTED, onConnected);
      web3Auth.off(ADAPTER_EVENTS.DISCONNECTED, onDisconnected);
      web3Auth.off(ADAPTER_EVENTS.ERRORED, onErrored);
    };
  }, [web3Auth, trackEvent, walletsInstalled, clearTokenCache]);

  return null;
}

// Component that handles wagmi reconnection after mount
// This prevents setState-in-render errors by deferring reconnection
function WagmiReconnectHandler() {
  const { reconnect } = useReconnect();
  const [hasReconnected, setHasReconnected] = useState(false);

  useEffect(() => {
    if (hasReconnected) return;

    // Delay reconnect to ensure all components are mounted
    // This prevents React error: "Cannot update a component while rendering a different component"
    const timer = setTimeout(() => {
      reconnect();
      setHasReconnected(true);
    }, 100);

    return () => {
      clearTimeout(timer);
    };
  }, [reconnect, hasReconnected]);

  return null;
}

function WagmiConfigProvider({ children }: { children: ReactNode }) {
  const { web3Auth } = useWeb3Auth();
  const [mounted, setMounted] = useState(false);
  const [isCheckingSession, setIsCheckingSession] = useState(true);

  // Wait for Web3Auth to be fully ready before mounting Wagmi
  useEffect(() => {
    if (!web3Auth) {
      return;
    }

    // If already connected, we're good to go
    if (web3Auth.connected && web3Auth.provider) {
      setIsCheckingSession(false);
      return;
    }

    // Otherwise, wait for Web3Auth to finish connecting (session restoration)
    let hasConnected = false;

    const handleConnected = () => {
      hasConnected = true;
      setIsCheckingSession(false);
    };

    // Listen for connection event
    web3Auth.on(ADAPTER_EVENTS.CONNECTED, handleConnected);

    // Set a timeout to avoid blocking forever if no session exists
    const timeout = setTimeout(() => {
      if (!hasConnected) {
        setIsCheckingSession(false);
      }
    }, 1500);

    return () => {
      web3Auth.off(ADAPTER_EVENTS.CONNECTED, handleConnected);
      clearTimeout(timeout);
    };
  }, [web3Auth]);

  useEffect(() => {
    setMounted(true);
  }, []);

  const wagmiConfig = useMemo(() => {
    if (!web3Auth) return null;

    return appConfig.e2eTestMode
      ? createConfig({
          chains: E2E_TEST_CHAINS,
          multiInjectedProviderDiscovery: false,
          connectors: [
            web3auth({
              web3AuthInstance: web3Auth,
            }),
          ],
          transports: {
            [localL1Network.id]: http(localL1Network.rpcUrls.default.http[0], { batch: true }),
            [localL2Network.id]: http(localL2Network.rpcUrls.default.http[0], { batch: true }),
          },
        })
      : createConfig({
          chains: CHAINS,
          multiInjectedProviderDiscovery: false,
          connectors: [
            web3auth({
              web3AuthInstance: web3Auth,
            }),
          ],
          transports: generateWagmiTransports(CHAINS_IDS),
        });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [web3Auth]);

  function generateWagmiTransports(chainIds: (typeof CHAINS_IDS)[number][]) {
    return chainIds.reduce(
      (acc, chainId) => {
        acc[chainId] = generateWagmiTransport(chainId);
        return acc;
      },
      {} as Record<(typeof CHAINS_IDS)[number], ReturnType<typeof http>>,
    );
  }

  function generateWagmiTransport(chainId: (typeof CHAINS_IDS)[number]) {
    return http(CHAINS_RPC_URLS[chainId], { batch: true });
  }

  // Wait for Web3Auth initialization, mounting, and wagmi config
  if (!mounted || isCheckingSession || !wagmiConfig) {
    return null;
  }

  return (
    <WagmiProvider config={wagmiConfig} reconnectOnMount={false}>
      <WagmiReconnectHandler />
      {children}
    </WagmiProvider>
  );
}

export function Web3Provider({ children }: DynamicProviderProps) {
  return (
    <Web3AuthProvider config={web3AuthContextConfig}>
      <Web3AuthEventBridge />
      <WagmiConfigProvider>{children}</WagmiConfigProvider>
    </Web3AuthProvider>
  );
}
