import { Address } from "viem";

import { NATIVE_BRIDGE_SUPPORTED_CHAIN_IDS } from "@/constants/chains";

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
  toChainId: SupportedChainIds;
  messageServiceAddress: Address;
  tokenBridgeAddress: Address;
  gasLimitSurplus: bigint;
  profitMargin: bigint;

  /** @group CCTP adapter — required for cross-chain USDC transfers */
  cctpDomain: number;
  /** @group CCTP adapter */
  cctpTokenMessengerV2Address: Address;
  /** @group CCTP adapter */
  cctpMessageTransmitterV2Address: Address;

  /** @group Hyperlane adapter — required for mUSD transfers */
  hyperlanePortalLiteAddress?: Address;
  /** @group Hyperlane adapter */
  hyperlaneMailboxAddress?: Address;

  /** @group Native adapter — L1 yield provider for LST claiming (e.g. stETH) */
  yieldProviderAddress?: Address;
};
