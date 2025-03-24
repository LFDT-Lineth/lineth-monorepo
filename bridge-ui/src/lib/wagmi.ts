import { http, createConfig } from "wagmi";
import { CHAINS, CHAINS_IDS, CHAINS_RPC_URLS } from "@/constants";

export const config = createConfig({
  chains: CHAINS,
  multiInjectedProviderDiscovery: false,
  transports: generateWagmiTransports(CHAINS_IDS),
});

function generateWagmiTransports(chainIds: (typeof CHAINS_IDS)[number][]) {
  return chainIds.reduce(
    (acc, chainId) => {
      acc[chainId] = generateWagmiTransport(chainId);
      return acc;
    },
    {} as Record<(typeof chainIds)[number], ReturnType<typeof generateWagmiTransport>>,
  );
}

function generateWagmiTransport(chainId: (typeof CHAINS_IDS)[number]) {
  return http(CHAINS_RPC_URLS[chainId], { batch: true });
}

declare module "wagmi" {
  interface Register {
    config: typeof config;
  }
}
