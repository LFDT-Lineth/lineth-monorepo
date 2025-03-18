import { http, createConfig } from "wagmi";
import { linea, lineaSepolia, mainnet, sepolia } from "wagmi/chains";

export const chains = [mainnet, linea, lineaSepolia, sepolia] as const;
export const supportedChainIds = [mainnet.id, linea.id, lineaSepolia.id, sepolia.id] as const;

export type SupportedChainId = (typeof supportedChainIds)[number];

export const config = createConfig({
  chains,
  multiInjectedProviderDiscovery: false,
  transports: {
    [mainnet.id]: http(`https://mainnet.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
    [sepolia.id]: http(`https://sepolia.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
    [linea.id]: http(`https://linea-mainnet.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
    [lineaSepolia.id]: http(`https://linea-sepolia.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`, { batch: true }),
  },
});

declare module "wagmi" {
  interface Register {
    config: typeof config;
  }
}
