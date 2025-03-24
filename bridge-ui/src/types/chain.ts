import { NATIVE_BRIDGE_SUPPORTED_CHAIN_IDS } from "@/constants";
import { Address } from "viem";

export type SupportedChainIds = (typeof NATIVE_BRIDGE_SUPPORTED_CHAIN_IDS)[number];

export enum ChainLayer {
  L1 = "L1",
  L2 = "L2",
}

export type Chain = {
  id: SupportedChainIds;
  name: string;
  iconPath: string;
  nativeCurrency: { name: string; symbol: string; decimals: number };
  blockExplorers?: {
    [key: string]: {
      name: string;
      url: string;
      apiUrl?: string | undefined;
    };
    default: {
      name: string;
      url: string;
      apiUrl?: string | undefined;
    };
  };
  testnet: boolean;
  layer: ChainLayer;
  messageServiceAddress: Address;
  tokenBridgeAddress: Address;
  gasLimitSurplus: bigint;
  profitMargin: bigint;
  cctpDomain: number;
  cctpTokenMessengerV2Address: Address;
  cctpMessageTransmitterV2Address: Address;
};
