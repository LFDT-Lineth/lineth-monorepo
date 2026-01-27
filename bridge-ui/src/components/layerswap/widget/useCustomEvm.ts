import { useCallback, useMemo } from "react";

import { resolveWalletConnectorIcon, NetworkWithTokens, NetworkType } from "@layerswap/widget";
import {
  WalletConnectionProvider,
  Wallet,
  WalletConnectionProviderProps,
  InternalConnector,
} from "@layerswap/widget/types";
import { CONNECTOR_EVENTS } from "@web3auth/modal";
import { useWeb3Auth, useWeb3AuthConnect, useWeb3AuthDisconnect } from "@web3auth/modal/react";
import { useAccount } from "wagmi";

export default function useEVM({ networks }: WalletConnectionProviderProps): WalletConnectionProvider {
  const name = "EVM";
  const id = "evm";

  // wagmi
  const { connector: activeConnector, address: activeAddress, isConnected } = useAccount();
  const { web3Auth } = useWeb3Auth();
  const { connect } = useWeb3AuthConnect();
  const { disconnect } = useWeb3AuthDisconnect();

  // Gather the EVM‐type network names
  const evmNetworkNames = useMemo(
    () => networks.filter((n) => n.type === NetworkType.EVM).map((n) => n.name),
    [networks],
  );

  // Supported-networks
  const supportedNetworks = useMemo(
    () => ({
      asSource: evmNetworkNames,
      autofill: evmNetworkNames,
      withdrawal: evmNetworkNames,
    }),
    [evmNetworkNames],
  );

  // connectWallet: trigger Web3Auth connection
  const connectWallet = useCallback(async (): Promise<Wallet | undefined> => {
    try {
      // Disconnect if already connected
      if (isConnected) {
        await disconnect();
      }

      // Connect
      await connect();

      // Wait for Web3Auth connection and get address
      const address = await waitForWeb3AuthConnection(web3Auth);

      if (!address) {
        return undefined;
      }

      return resolveWallet({
        connectorId: "web3auth",
        connectorName: "Web3Auth",
        address,
        isActive: true,
        networks,
        supportedNetworks,
        disconnect: async () => disconnect(),
        providerName: name,
      });
    } catch (error) {
      console.error("Failed to connect wallet:", error);
      return undefined;
    }
  }, [isConnected, connect, web3Auth, networks, supportedNetworks, disconnect, name]);

  // Logout
  const disconnectWallets = useCallback(async () => {
    disconnect();
  }, [disconnect]);

  // Map connected wallet to Layerswap Wallet shape
  const connectedWallets: Wallet[] = useMemo(() => {
    if (!isConnected || !activeAddress || !activeConnector) {
      return [];
    }

    const wallet = resolveWallet({
      connectorId: activeConnector.id,
      connectorName: activeConnector.name,
      address: activeAddress,
      isActive: true,
      networks,
      supportedNetworks,
      disconnect: disconnectWallets,
      providerName: name,
    });

    return wallet ? [wallet] : [];
  }, [isConnected, activeAddress, activeConnector, networks, supportedNetworks, disconnectWallets]);

  const logo = networks.find((n) => n.name.toLowerCase().includes("linea"))?.logo;
  const availableConnectors: InternalConnector[] = [
    {
      id: "web3auth",
      name: "Web3Auth",
      icon: logo,
      providerName: name,
    },
  ];

  return {
    connectWallet,
    availableWalletsForConnect: availableConnectors,
    activeWallet: connectedWallets.find((w) => w.isActive),
    connectedWallets,
    asSourceSupportedNetworks: supportedNetworks.asSource,
    autofillSupportedNetworks: supportedNetworks.autofill,
    withdrawalSupportedNetworks: supportedNetworks.withdrawal,
    name,
    id,
    providerIcon: logo,
  };
}

/** Reusable helper to turn a wagmi connector + address into Layerswap `Wallet` shape */
function resolveWallet(props: {
  connectorId: string;
  connectorName: string;
  address: string;
  isActive: boolean;
  networks: NetworkWithTokens[];
  supportedNetworks: {
    asSource: string[];
    autofill: string[];
    withdrawal: string[];
  };
  disconnect: () => Promise<void>;
  providerName: string;
}): Wallet | undefined {
  const { connectorId, connectorName, address, isActive, networks, supportedNetworks, disconnect, providerName } =
    props;

  if (!address) return;

  const displayName = `${connectorName} – ${providerName}`;
  const networkIcon = networks.find((n) => n.name.toLowerCase().includes("linea"))?.logo;

  return {
    id: connectorId,
    isActive,
    address,
    addresses: [address],
    displayName,
    providerName,
    icon: resolveWalletConnectorIcon({ connector: connectorName, address }),
    disconnect: () => disconnect(),
    asSourceSupportedNetworks: supportedNetworks.asSource,
    autofillSupportedNetworks: supportedNetworks.autofill,
    withdrawalSupportedNetworks: supportedNetworks.withdrawal,
    networkIcon,
  };
}

/**
 * Helper function to wait for Web3Auth connection and get account address
 * @param web3Auth - Web3Auth instance
 * @param timeout - Maximum time to wait in milliseconds (default: 30000)
 * @returns Promise that resolves with the account address or rejects on timeout
 */
function waitForWeb3AuthConnection(
  web3Auth: ReturnType<typeof useWeb3Auth>["web3Auth"],
  timeout: number = 30000,
): Promise<string> {
  return new Promise((resolve, reject) => {
    if (!web3Auth) {
      reject(new Error("Web3Auth instance not available"));
      return;
    }

    let web3AuthUnsubscribe: (() => void) | undefined;

    const cleanup = () => {
      if (web3AuthUnsubscribe) {
        web3AuthUnsubscribe();
      }
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
    };

    // Set up timeout
    const timeoutId = setTimeout(() => {
      cleanup();
      reject(new Error("Timeout waiting for wallet connection"));
    }, timeout);

    // Get address from Web3Auth provider
    const getAddressFromProvider = async (): Promise<string | null> => {
      try {
        const provider = web3Auth.provider;
        if (!provider) {
          return null;
        }

        // Request accounts from the provider
        const accounts = (await provider.request({
          method: "eth_accounts",
        })) as string[];

        if (accounts && Array.isArray(accounts) && accounts.length > 0) {
          return accounts[0];
        }

        return null;
      } catch (error) {
        console.error("Error getting address from Web3Auth provider:", error);
        return null;
      }
    };

    // Handle Web3Auth connection
    const onConnected = async () => {
      if (web3AuthUnsubscribe) {
        web3AuthUnsubscribe();
        web3AuthUnsubscribe = undefined;
      }

      // Get address directly from Web3Auth provider
      const address = await getAddressFromProvider();
      if (address) {
        cleanup();
        resolve(address);
      } else {
        // If address not immediately available, wait a bit and retry
        setTimeout(async () => {
          const retryAddress = await getAddressFromProvider();
          if (retryAddress) {
            cleanup();
            resolve(retryAddress);
          } else {
            cleanup();
            reject(new Error("Failed to get account address from Web3Auth"));
          }
        }, 500);
      }
    };

    const onErrored = () => {
      cleanup();
      reject(new Error("Web3Auth connection error"));
    };

    // Listen to Web3Auth events
    web3Auth.on(CONNECTOR_EVENTS.CONNECTED, onConnected);
    web3Auth.on(CONNECTOR_EVENTS.ERRORED, onErrored);

    web3AuthUnsubscribe = () => {
      web3Auth.off(CONNECTOR_EVENTS.CONNECTED, onConnected);
      web3Auth.off(CONNECTOR_EVENTS.ERRORED, onErrored);
    };

    // Check if already connected
    if (web3Auth.connected) {
      onConnected();
    }
  });
}
