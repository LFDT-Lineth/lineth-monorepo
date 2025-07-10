import { http, createConfig } from "wagmi";
import { CHAINS, CHAINS_IDS, E2E_TEST_CHAINS, CHAINS_RPC_URLS, localL1Network, localL2Network } from "@/constants";
import { config as appConfig } from "@/config";

export const config = appConfig.e2eTestMode
  ? createConfig({
      chains: E2E_TEST_CHAINS,
      multiInjectedProviderDiscovery: false,
      transports: {
        [localL1Network.id]: http(localL1Network.rpcUrls.default.http[0], { batch: true }),
        [localL2Network.id]: http(localL2Network.rpcUrls.default.http[0], { batch: true }),
      },
    })
  : createConfig({
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
    {} as Record<(typeof CHAINS_IDS)[number], ReturnType<typeof http>>,
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
