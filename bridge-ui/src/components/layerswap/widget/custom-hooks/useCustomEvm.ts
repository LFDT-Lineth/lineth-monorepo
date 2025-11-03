import { useAccount, useConnectors } from "wagmi";
import { useCallback, useMemo } from "react";
import {
  WalletProvider,
  Wallet,
  resolveWalletConnectorIcon,
  useSettingsState,
  NetworkWithTokens,
  NetworkType,
} from "@layerswap/widget";
import { useWeb3AuthConnect, useWeb3AuthDisconnect } from "@web3auth/modal/react";

export default function useEVM(): WalletProvider {
  const name = "EVM";
  const id = "evm";

  // wagmi
  const { connector: activeConnector, address: activeAddress, isConnected } = useAccount();
  const { connect } = useWeb3AuthConnect();
  const { disconnect } = useWeb3AuthDisconnect();
  const connectors = useConnectors();

  // Layerswap settings
  const { networks } = useSettingsState();

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
        disconnect();
      }

      // Find Web3Auth connector
      const web3authConnector = connectors.find((c) => c.id === "web3auth");
      if (!web3authConnector) {
        throw new Error("Web3Auth connector not found");
      }

      // Connect
      await connect();

      if (!activeAddress) {
        return undefined;
      }

      return resolveWallet({
        connectorId: web3authConnector.id,
        connectorName: web3authConnector.name,
        address: activeAddress,
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
  }, [isConnected, connectors, connect, activeAddress, networks, supportedNetworks, disconnect]);

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

  return {
    connectWallet,
    connectConnector: connectWallet,
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
