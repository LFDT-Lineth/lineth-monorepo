import { Address } from "viem";
import { linea, mainnet, Chain as ViemChain, sepolia, lineaSepolia } from "viem/chains";
import { SupportedChainId } from "@/lib/wagmi";
import { config } from "@/config";
import { Chain, ChainLayer } from "@/types";

export const generateChain = (chain: ViemChain): Chain => {
  return {
    id: chain.id as SupportedChainId,
    name: chain.id !== lineaSepolia.id ? chain.name : "Linea Sepolia",
    iconPath: config.chains[chain.id].iconPath,
    nativeCurrency: chain.nativeCurrency,
    blockExplorers: chain.blockExplorers,
    // Possibly the wrong assumption to fallback to 'false', but fallback to 'true' makes the app crash mysteriously
    testnet: Boolean(chain.testnet),
    layer: getChainNetworkLayer(chain.id),
    messageServiceAddress: config.chains[chain.id].messageServiceAddress as Address,
    tokenBridgeAddress: config.chains[chain.id].tokenBridgeAddress as Address,
    gasLimitSurplus: config.chains[chain.id].gasLimitSurplus,
    profitMargin: config.chains[chain.id].profitMargin,
    cctpDomain: config.chains[chain.id].cctpDomain,
    cctpTokenMessengerV2Address: config.chains[chain.id].cctpTokenMessengerV2Address as Address,
    cctpMessageTransmitterV2Address: config.chains[chain.id].cctpMessageTransmitterV2Address as Address,
  };
};

export const generateChains = (chains: ViemChain[]): Chain[] => {
  return chains.map(generateChain);
};

export const getChainNetworkLayer = (chainId: number) => {
  switch (chainId) {
    case linea.id:
    case lineaSepolia.id:
      return ChainLayer.L2;
    case mainnet.id:
    case sepolia.id:
      return ChainLayer.L1;
    default:
      throw new Error(`Unsupported chain id: ${chainId}`);
  }
};

export const getChainLogoPath = (chainId: number) => {
  switch (chainId) {
    case linea.id:
      return "/images/logo/linea-rounded.svg";
    case lineaSepolia.id:
      return "/images/logo/linea-sepolia.svg";
    case mainnet.id:
    case sepolia.id:
      return "/images/logo/ethereum-rounded.svg";
    default:
      return "";
  }
};
