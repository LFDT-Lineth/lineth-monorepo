"use client";

import { ReactNode, useEffect } from "react";

import { CONNECTOR_EVENTS, WALLET_CONNECTORS, WEB3AUTH_NETWORK } from "@web3auth/modal";
import { coinbaseConnector } from "@web3auth/modal/connectors/coinbase-connector";
import { useWeb3Auth, Web3AuthContextConfig, Web3AuthProvider } from "@web3auth/modal/react";
import { WagmiProvider } from "@web3auth/modal/react/wagmi";

import { config as appConfig } from "@/config";
import { useCachedIdentityToken } from "@/hooks/useCachedIdentityToken";
import useGTM from "@/hooks/useGtm";

import { useWalletDetection } from "./WalletDetectionProvider";
import { isProd } from "../../next.config";

interface DynamicProviderProps {
  children: ReactNode;
}

const clientId = appConfig.web3AuthClientId;

const googleAuthConnectionId = process.env.NEXT_PUBLIC_CONNECTOR_GOOGLE_ID || "";
const passwordlessAuthConnectionId = process.env.NEXT_PUBLIC_CONNECTOR_PASSWORDLESS_ID || "";
const groupedAuthConnectionId = process.env.NEXT_PUBLIC_CONNECTOR_GROUPED_ID || "";

const isSocialLoginEnabled = process.env.NEXT_PUBLIC_SOCIAL_LOGIN_ENABLED === "true";

const connectorLabel = "Web3Auth";

const socialLoginDisabledConfig = {
  label: connectorLabel,
  showOnModal: false,
};

const socialLoginEnabledConfig = {
  label: connectorLabel,
  loginMethods: {
    google: {
      name: "Google",
      description: "Continue with Google",
      mainOption: true,
      authConnectionId: googleAuthConnectionId,
      groupedAuthConnectionId: groupedAuthConnectionId,
    },
    email_passwordless: {
      name: "Email Passwordless login",
      authConnectionId: passwordlessAuthConnectionId,
      groupedAuthConnectionId: groupedAuthConnectionId,
    },
  },
};

const web3AuthContextConfig: Web3AuthContextConfig = {
  web3AuthOptions: {
    initialAuthenticationMode: "connect-and-sign",
    clientId,
    web3AuthNetwork: isProd ? WEB3AUTH_NETWORK.SAPPHIRE_MAINNET : WEB3AUTH_NETWORK.SAPPHIRE_DEVNET,
    defaultChainId: "0xe708",
    uiConfig: {
      appUrl: "https://linea.build/hub/bridge",
      displayErrorsOnModal: true,
    },
    // Coinbase connector supports Linea chain only for EOA wallets
    // Coinbase's smart wallets make the switch chain throw an error
    connectors: [coinbaseConnector({ options: "eoaOnly" })],
    modalConfig: {
      connectors: {
        [WALLET_CONNECTORS.AUTH]: isSocialLoginEnabled ? socialLoginEnabledConfig : socialLoginDisabledConfig,
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

    const onAuthorized = (args: { connector: string }) => {
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
    web3Auth.on(CONNECTOR_EVENTS.AUTHORIZED, onAuthorized);
    web3Auth.on(CONNECTOR_EVENTS.DISCONNECTED, onDisconnected);
    web3Auth.on(CONNECTOR_EVENTS.ERRORED, onErrored);

    return () => {
      web3Auth.off(CONNECTOR_EVENTS.CONNECTING, onConnecting);
      web3Auth.off(CONNECTOR_EVENTS.AUTHORIZED, onAuthorized);
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
