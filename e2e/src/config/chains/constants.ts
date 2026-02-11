import { defineChain } from "viem";

export const localL1Network = defineChain({
  id: 31648428,
  name: "Local L1 Network",
  nativeCurrency: {
    decimals: 18,
    name: "Ether",
    symbol: "ETH",
  },
  rpcUrls: {
    default: {
      http: ["http://127.0.0.1:8445"],
      webSocket: ["ws://127.0.0.1:8445"],
    },
  },
  testnet: true,
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
  rpcUrls: {
    default: {
      http: ["http://127.0.0.1:9045"],
      webSocket: ["ws://127.0.0.1:9045"],
    },
  },
  testnet: true,
});

export const lineaDevnet = defineChain({
  id: 59139,
  name: "Linea Devnet",
  nativeCurrency: {
    decimals: 18,
    name: "Ether",
    symbol: "ETH",
  },
  blockExplorers: {
    default: {
      name: "Lineascan",
      url: "https://lineascan.build",
      apiUrl: "https://api.lineascan.build/api",
    },
  },
  rpcUrls: {
    default: {
      http: ["https://rpc.devnet.linea.build"],
      webSocket: ["wss://rpc.devnet.linea.build"],
    },
  },
  testnet: true,
});
