import { Address } from "viem";
import { linea, mainnet, Chain as ViemChain, sepolia, lineaSepolia } from "viem/chains";

import { config } from "@/config";
import { localL1Network, localL2Network } from "@/constants/chains";
import { Chain, ChainLayer, SupportedChainIds } from "@/types";

const getChainName = (chainId: number) => {
  switch (chainId) {
    case linea.id:
      return "Linea";
    case lineaSepolia.id:
      return "Linea Sepolia";
    case mainnet.id:
      return "Ethereum";
    case sepolia.id:
      return "Sepolia";
    case localL1Network.id:
      return "Local L1 Network";
    case localL2Network.id:
      return "Local L2 Network";
    default:
      return "";
  }
};

export const generateChain = (chain: ViemChain): Chain => {
  return {
    id: chain.id as SupportedChainIds,
    name: getChainName(chain.id),
    iconPath: config.chains[chain.id].iconPath,
    nativeCurrency: chain.nativeCurrency,
    blockExplorers: chain.blockExplorers,
    testnet: Boolean(chain.testnet),
    layer: getChainNetworkLayer(chain.id),
    toChainId: getDestinationChainId(chain.id),
    messageServiceAddress: config.chains[chain.id].messageServiceAddress as Address,
    tokenBridgeAddress: config.chains[chain.id].tokenBridgeAddress as Address,
    gasLimitSurplus: config.chains[chain.id].gasLimitSurplus,
    profitMargin: config.chains[chain.id].profitMargin,
    cctpDomain: config.chains[chain.id].cctpDomain,
    cctpTokenMessengerV2Address: config.chains[chain.id].cctpTokenMessengerV2Address as Address,
    cctpMessageTransmitterV2Address: config.chains[chain.id].cctpMessageTransmitterV2Address as Address,
    // Optional field for local networks for testing purposes
    ...(chain.custom?.localNetwork ? { localNetwork: true } : {}),
  };
};

export const generateChains = (chains: ViemChain[]): Chain[] => {
  return chains.map(generateChain);
};

export const getChainNetworkLayer = (chainId: number) => {
  // For non-local networks, we can safely assume the layer based on the chain ID
  switch (chainId) {
    case linea.id:
    case lineaSepolia.id:
      return ChainLayer.L2;
    case mainnet.id:
    case sepolia.id:
      return ChainLayer.L1;
    case localL1Network.id:
      return ChainLayer.L1;
    case localL2Network.id:
      return ChainLayer.L2;
    default:
      throw new Error(`Unsupported chain id: ${chainId}`);
  }
};

export const getDestinationChainId = (chainId: number): SupportedChainIds => {
  switch (chainId) {
    case linea.id:
      return mainnet.id;
    case lineaSepolia.id:
      return sepolia.id;
    case mainnet.id:
      return linea.id;
    case sepolia.id:
      return lineaSepolia.id;
    case localL1Network.id:
      return localL2Network.id;
    case localL2Network.id:
      return localL1Network.id;
    default:
      throw new Error(`Unsupported chain id: ${chainId}`);
  }
};

export const getChainLogoPath = (chainId: number) => {
  switch (chainId) {
    case linea.id:
      return config.chains[linea.id].iconPath;
    case lineaSepolia.id:
      return config.chains[lineaSepolia.id].iconPath;
    case mainnet.id:
    case sepolia.id:
      return config.chains[mainnet.id].iconPath;
    case localL1Network.id:
      return config.chains[localL1Network.id].iconPath;
    case localL2Network.id:
      return config.chains[localL2Network.id].iconPath;
    default:
      return "";
  }
};
