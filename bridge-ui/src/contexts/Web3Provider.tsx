"use client";

import { ReactNode, useEffect } from "react";

import { WEB3AUTH_NETWORK, CONNECTOR_EVENTS } from "@web3auth/modal";
import { coinbaseConnector } from "@web3auth/modal/connectors/coinbase-connector";
import { useWeb3Auth, Web3AuthContextConfig, Web3AuthProvider } from "@web3auth/modal/react";
import { WagmiProvider } from "@web3auth/modal/react/wagmi";
import { toHex } from "viem";

import { config as appConfig } from "@/config";
import { localL1Network, localL2Network } from "@/constants/chains";
import { useCachedIdentityToken } from "@/hooks/useCachedIdentityToken";
import useGTM from "@/hooks/useGtm";

import { useWalletDetection } from "./WalletDetectionProvider";
import { isProd } from "../../next.config";

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
    ...(appConfig.e2eTestMode === true
      ? {
          chains: [
            {
              chainNamespace: "eip155",
              logo: "https://images.web3auth.io/chains/1.svg",
              displayName: localL1Network.name,
              tickerName: localL1Network.nativeCurrency.name,
              ticker: localL1Network.nativeCurrency.symbol,
              chainId: toHex(localL1Network.id),
              rpcTarget: localL1Network.rpcUrls.default.http[0],
              blockExplorerUrl: localL1Network.blockExplorers?.default.url,
            },
            {
              chainNamespace: "eip155",
              logo: "https://images.web3auth.io/chains/59144.svg",
              displayName: localL2Network.name,
              tickerName: localL2Network.nativeCurrency.name,
              ticker: localL2Network.nativeCurrency.symbol,
              chainId: toHex(localL2Network.id),
              rpcTarget: localL2Network.rpcUrls.default.http[0],
              blockExplorerUrl: localL2Network.blockExplorers?.default.url,
            },
          ],
        }
      : {}),
    // coinbase connector supports linea chain only for eoa wallets
    // if we try to use it with smartWallets, switch chain throws an error.
    connectors: [coinbaseConnector({ options: "eoaOnly" })],
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

    web3Auth.on(CONNECTOR_EVENTS.CONNECTING, onConnecting);
    web3Auth.on(CONNECTOR_EVENTS.CONNECTED, onConnected);
    web3Auth.on(CONNECTOR_EVENTS.DISCONNECTED, onDisconnected);
    web3Auth.on(CONNECTOR_EVENTS.ERRORED, onErrored);

    return () => {
      web3Auth.off(CONNECTOR_EVENTS.CONNECTING, onConnecting);
      web3Auth.off(CONNECTOR_EVENTS.CONNECTED, onConnected);
      web3Auth.off(CONNECTOR_EVENTS.DISCONNECTED, onDisconnected);
      web3Auth.off(CONNECTOR_EVENTS.ERRORED, onErrored);
    };
  }, [web3Auth, trackEvent, walletsInstalled, clearTokenCache]);

  return null;
}

export function Web3Provider({ children }: DynamicProviderProps) {
  return (
    <Web3AuthProvider config={web3AuthContextConfig}>
      <Web3AuthEventBridge />
      <WagmiProvider>{children}</WagmiProvider>
    </Web3AuthProvider>
  );
}
