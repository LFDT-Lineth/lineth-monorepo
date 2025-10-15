import type { CreateConnectorFn } from "wagmi";
import { createConnector } from "wagmi";
import type { Web3Auth } from "@web3auth/modal";

export interface Web3AuthConnectorConfig {
  web3AuthInstance: Web3Auth;
}

// Global map to track if listeners are already setup for a provider
// This prevents duplicate listeners when connector is recreated
const providerListeners = new WeakMap<any, boolean>();

/**
 * Custom Wagmi connector for Web3Auth Modal
 * This ensures all wallet connections go through Web3Auth authentication
 */
export function web3auth(config: Web3AuthConnectorConfig): CreateConnectorFn {
  type Provider = any;
  let connectedProvider: Provider | null = null;

  return createConnector<Provider>((wagmiConfig) => ({
    id: "web3auth",
    name: "Web3Auth",
    type: "web3auth",

    async connect() {
      const web3Auth = config.web3AuthInstance;

      // If not connected, open Web3Auth modal
      if (!web3Auth.connected) {
        await web3Auth.connect();
      }

      connectedProvider = web3Auth.provider as Provider;

      // Setup event listeners only once per provider instance
      if (connectedProvider && !providerListeners.has(connectedProvider)) {
        providerListeners.set(connectedProvider, true);

        const onAccountsChanged = (accounts: string[]) => {
          if (accounts.length === 0) {
            wagmiConfig.emitter.emit("disconnect");
          } else {
            wagmiConfig.emitter.emit("change", {
              accounts: accounts.map((x) => x as `0x${string}`),
            });
          }
        };

        const onChainChanged = (chainId: string | number) => {
          const id = Number(chainId);
          wagmiConfig.emitter.emit("change", { chainId: id });
        };

        const onDisconnect = () => {
          wagmiConfig.emitter.emit("disconnect");
        };

        connectedProvider.on("accountsChanged", onAccountsChanged);
        connectedProvider.on("chainChanged", onChainChanged);
        connectedProvider.on("disconnect", onDisconnect);
      }

      // Get accounts
      const accounts = await this.getAccounts();
      const currentChainId = await this.getChainId();

      return {
        accounts,
        chainId: currentChainId,
      };
    },

    async disconnect() {
      const web3Auth = config.web3AuthInstance;

      if (web3Auth.connected) {
        await web3Auth.logout();
      }

      // Clear provider reference but keep listeners registered
      // (they will be cleaned up when provider is garbage collected)
      connectedProvider = null;
    },

    async getAccounts() {
      try {
        const provider = await this.getProvider();
        if (!provider) {
          return [];
        }

        const accounts = (await provider.request({
          method: "eth_accounts",
        })) as string[];

        return accounts.map((x) => x as `0x${string}`);
      } catch (error) {
        console.error("[Web3AuthConnector] getAccounts error:", error);
        return [];
      }
    },

    async getChainId() {
      const provider = await this.getProvider();
      if (!provider) throw new Error("Provider not available");

      const chainId = await provider.request({ method: "eth_chainId" });
      return Number(chainId);
    },

    async getProvider() {
      const web3Auth = config.web3AuthInstance;
      if (!web3Auth.provider) {
        throw new Error("Web3Auth provider not available");
      }
      return web3Auth.provider as any;
    },

    async isAuthorized() {
      try {
        const web3Auth = config.web3AuthInstance;

        // Check if Web3Auth has an active session
        if (!web3Auth.connected || !web3Auth.provider) {
          return false;
        }

        const accounts = await this.getAccounts();

        return accounts.length > 0;
      } catch (error) {
        console.error("[Web3AuthConnector] isAuthorized error:", error);
        return false;
      }
    },

    async switchChain({ chainId }) {
      const provider = await this.getProvider();
      const chain = wagmiConfig.chains.find((c) => c.id === chainId);

      if (!chain) {
        throw new Error(`Chain ${chainId} not configured`);
      }

      try {
        await provider.request({
          method: "wallet_switchEthereumChain",
          params: [{ chainId: `0x${chainId.toString(16)}` }],
        });

        return chain;
      } catch (error: any) {
        // If chain doesn't exist in wallet, try to add it
        if (error.code === 4902 || error.code === -32603) {
          await provider.request({
            method: "wallet_addEthereumChain",
            params: [
              {
                chainId: `0x${chainId.toString(16)}`,
                chainName: chain.name,
                nativeCurrency: chain.nativeCurrency,
                rpcUrls: [chain.rpcUrls.default.http[0]],
                blockExplorerUrls: chain.blockExplorers ? [chain.blockExplorers.default.url] : undefined,
              },
            ],
          });
          return chain;
        }
        throw error;
      }
    },

    onAccountsChanged(accounts: string[]) {
      if (accounts.length === 0) {
        this.onDisconnect();
      } else {
        wagmiConfig.emitter.emit("change", {
          accounts: accounts.map((x) => x as `0x${string}`),
        });
      }
    },

    onChainChanged(chainId: string | number) {
      const id = Number(chainId);
      wagmiConfig.emitter.emit("change", { chainId: id });
    },

    onDisconnect() {
      wagmiConfig.emitter.emit("disconnect");
      connectedProvider = null;
    },
  }));
}
