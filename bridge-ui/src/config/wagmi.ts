import { http } from "@wagmi/core";
import { mainnet, sepolia, linea, lineaSepolia } from "@wagmi/core/chains";
import { config } from "./config";
import { WagmiAdapter } from "@reown/appkit-adapter-wagmi";
import { AppKitNetwork } from "@reown/appkit/networks";

if (!config.walletConnectId) throw new Error("Project ID is not defined");

export const chains: [AppKitNetwork, ...AppKitNetwork[]] = [mainnet, sepolia, linea, lineaSepolia];

export const wagmiAdapter = new WagmiAdapter({
  networks: chains,
  projectId: config.walletConnectId,
  multiInjectedProviderDiscovery: true,
  ssr: true,

  batch: {
    multicall: true,
  },
  transports: {
    [mainnet.id]: http(`https://mainnet.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
    [sepolia.id]: http(`https://sepolia.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
    [linea.id]: http(`https://linea-mainnet.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
    [lineaSepolia.id]: http(`https://linea-sepolia.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
  },
});

export const wagmiConfig = wagmiAdapter.wagmiConfig;
