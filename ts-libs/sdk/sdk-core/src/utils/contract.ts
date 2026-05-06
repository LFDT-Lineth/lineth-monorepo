import { isLineaMainnet, isLineaSepolia, isMainnet, isSepolia } from "./chain";
import {
  L2_MESSAGE_SERVICE_MAINNET_ADDRESS,
  L2_MESSAGE_SERVICE_SEPOLIA_ADDRESS,
  LINEA_ROLLUP_MAINNET_ADDRESS,
  LINEA_ROLLUP_SEPOLIA_ADDRESS,
  TOKEN_BRIDGE_LINEA_MAINNET_ADDRESS,
  TOKEN_BRIDGE_LINEA_SEPOLIA_ADDRESS,
  TOKEN_BRIDGE_MAINNET_ADDRESS,
  TOKEN_BRIDGE_SEPOLIA_ADDRESS,
} from "../constants/address";
import { Address } from "../types/misc";

export function getContractsAddressesByChainId(chainId: number): {
  messageService: Address;
  destinationChainMessageService: Address;
  tokenBridge: Address;
} {
  if (isMainnet(chainId)) {
    return {
      messageService: LINEA_ROLLUP_MAINNET_ADDRESS,
      destinationChainMessageService: L2_MESSAGE_SERVICE_MAINNET_ADDRESS,
      tokenBridge: TOKEN_BRIDGE_MAINNET_ADDRESS,
    };
  }

  if (isSepolia(chainId)) {
    return {
      messageService: LINEA_ROLLUP_SEPOLIA_ADDRESS,
      destinationChainMessageService: L2_MESSAGE_SERVICE_SEPOLIA_ADDRESS,
      tokenBridge: TOKEN_BRIDGE_SEPOLIA_ADDRESS,
    };
  }

  if (isLineaMainnet(chainId)) {
    return {
      messageService: L2_MESSAGE_SERVICE_MAINNET_ADDRESS,
      destinationChainMessageService: LINEA_ROLLUP_MAINNET_ADDRESS,
      tokenBridge: TOKEN_BRIDGE_LINEA_MAINNET_ADDRESS,
    };
  }

  if (isLineaSepolia(chainId)) {
    return {
      messageService: L2_MESSAGE_SERVICE_SEPOLIA_ADDRESS,
      destinationChainMessageService: LINEA_ROLLUP_SEPOLIA_ADDRESS,
      tokenBridge: TOKEN_BRIDGE_LINEA_SEPOLIA_ADDRESS,
    };
  }

  throw new Error("Unsupported chain ID. Only Ethereum and Linea networks are supported.");
}
