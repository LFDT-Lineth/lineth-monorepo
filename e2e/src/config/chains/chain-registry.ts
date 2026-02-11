import { lineaSepolia, sepolia } from "viem/chains";

import { lineaDevnet, localL1Network, localL2Network } from "./constants";

export function resolveChain(chainId: number) {
  switch (chainId) {
    case 11155111:
      return sepolia;

    case 59139:
      return lineaDevnet;

    case 59141:
      return lineaSepolia;

    case 31648428:
      return localL1Network;

    case 1337:
      return localL2Network;

    default:
      throw new Error(`Unsupported chain ID: ${chainId}`);
  }
}
