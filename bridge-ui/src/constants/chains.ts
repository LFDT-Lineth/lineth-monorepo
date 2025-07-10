import { config } from "@/config";
import { defineChain } from "viem";

// This is a local L1 network configuration for testing purposes
export const localL1Network = defineChain({
  id: 31648428,
  name: "Local L1 Network",
  nativeCurrency: {
    decimals: 18,
    name: "Ether",
    symbol: "ETH",
  },
  blockExplorers: {
    default: {
      name: "Etherscan",
      url: "https://etherscan.io",
      apiUrl: "https://api.etherscan.io/api",
    },
  },
  rpcUrls: {
    default: {
      http: ["http://127.0.0.1:8445"],
      webSocket: ["ws://127.0.0.1:8445"],
    },
  },
  testnet: true,
  custom: {
    localNetwork: true,
  },
});

// This is a local L2 Network configuration for testing purposes
export const localL2Network = defineChain({
  id: 1337,
  name: "Local L2 Network",
  nativeCurrency: {
    decimals: 18,
    name: "Ether",
    symbol: "ETH",
  },
  blockExplorers: {
    default: {
      name: "Etherscan",
      url: "https://lineascan.build",
      apiUrl: "https://api.lineascan.build/api",
    },
  },
  rpcUrls: {
    default: {
      http: ["http://127.0.0.1:9045"],
      webSocket: ["ws://127.0.0.1:9045"],
    },
  },
  testnet: true,
  custom: {
    localNetwork: true,
  },
});

import {
  arbitrum,
  aurora,
  avalanche,
  base,
  blast,
  bsc,
  celo,
  cronos,
  fantom,
  gnosis,
  ink,
  linea,
  lineaSepolia,
  mainnet,
  mantle,
  mode,
  moonbeam,
  optimism,
  polygon,
  scroll,
  sei,
  sepolia,
  sonic,
  zksync,
} from "viem/chains";

export const CHAINS = [
  mainnet,
  sepolia,
  linea,
  lineaSepolia,
  arbitrum,
  aurora,
  avalanche,
  base,
  blast,
  bsc,
  celo,
  cronos,
  fantom,
  gnosis,
  ink,
  mantle,
  mode,
  moonbeam,
  optimism,
  polygon,
  scroll,
  sei,
  sonic,
  zksync,
] as const;

export const E2E_TEST_CHAINS = [localL1Network, localL2Network] as const;
const SOLANA_CHAIN = 1151111081099710 as const;

export const CHAINS_IDS = [...CHAINS.map((chain) => chain.id), SOLANA_CHAIN];

export const CHAINS_RPC_URLS: Record<(typeof CHAINS_IDS)[number], string> = {
  [mainnet.id]: `https://mainnet.infura.io/v3/${config.infuraApiKey}`,
  [sepolia.id]: `https://sepolia.infura.io/v3/${config.infuraApiKey}`,
  [linea.id]: `https://linea-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [lineaSepolia.id]: `https://linea-sepolia.infura.io/v3/${config.infuraApiKey}`,
  [arbitrum.id]: `https://arbitrum-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [aurora.id]: `https://mainnet.aurora.dev`,
  [avalanche.id]: `https://avalanche-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [base.id]: `https://base-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [blast.id]: `https://blast-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [bsc.id]: `https://bsc-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [celo.id]: `https://celo-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [cronos.id]: `https://evm.cronos.org`,
  [fantom.id]: `https://rpc.ankr.com/fantom`,
  [gnosis.id]: `https://rpc.gnosischain.com`,
  [ink.id]: `https://rpc-gel.inkonchain.com`,
  [mantle.id]: `https://mantle-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [mode.id]: `https://mainnet.mode.network`,
  [moonbeam.id]: `https://rpc.testnet.moonbeam.network`,
  [optimism.id]: `https://optimism-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [polygon.id]: `https://polygon-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [scroll.id]: `https://scroll-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [sei.id]: `https://evm-rpc.sei-apis.com`,
  [sonic.id]: `https://rpc.soniclabs.com`,
  [zksync.id]: `https://zksync-mainnet.infura.io/v3/${config.infuraApiKey}`,
  [SOLANA_CHAIN]: `https://old-light-county.solana-mainnet.quiknode.pro/${config.quickNodeApiKey}`,
};

export const NATIVE_BRIDGE_SUPPORTED_CHAIN_IDS = [
  mainnet.id,
  linea.id,
  lineaSepolia.id,
  sepolia.id,
  // Local networks for testing purposes
  ...(config.e2eTestMode ? [localL1Network.id, localL2Network.id] : []),
] as const;
