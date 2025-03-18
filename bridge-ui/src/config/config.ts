import { getAddress } from "viem";
import { configSchema, Config } from "./config.schema";

export const config: Config = {
  chains: {
    1: {
      iconPath: "/images/logo/ethereum-rounded.svg",
      messageServiceAddress: getAddress(process.env.NEXT_PUBLIC_MAINNET_L1_MESSAGE_SERVICE ?? ""),
      tokenBridgeAddress: getAddress(process.env.NEXT_PUBLIC_MAINNET_L1_TOKEN_BRIDGE ?? ""),
      gasLimitSurplus: process.env.NEXT_PUBLIC_MAINNET_DEFAULT_GAS_LIMIT_SURPLUS
        ? BigInt(process.env.NEXT_PUBLIC_MAINNET_DEFAULT_GAS_LIMIT_SURPLUS)
        : BigInt(6000),
      profitMargin: process.env.NEXT_PUBLIC_MAINNET_PROFIT_MARGIN
        ? BigInt(process.env.NEXT_PUBLIC_MAINNET_PROFIT_MARGIN)
        : BigInt(1),
      cctpDomain: 0,
    },
    59144: {
      iconPath: "/images/logo/linea-mainnet.svg",
      messageServiceAddress: getAddress(process.env.NEXT_PUBLIC_MAINNET_LINEA_MESSAGE_SERVICE ?? ""),
      tokenBridgeAddress: getAddress(process.env.NEXT_PUBLIC_MAINNET_LINEA_TOKEN_BRIDGE ?? ""),
      gasLimitSurplus: process.env.NEXT_PUBLIC_MAINNET_DEFAULT_GAS_LIMIT_SURPLUS
        ? BigInt(process.env.NEXT_PUBLIC_MAINNET_DEFAULT_GAS_LIMIT_SURPLUS)
        : BigInt(6000),
      profitMargin: process.env.NEXT_PUBLIC_MAINNET_PROFIT_MARGIN
        ? BigInt(process.env.NEXT_PUBLIC_MAINNET_PROFIT_MARGIN)
        : BigInt(1),
      cctpDomain: 11,
    },
    11155111: {
      iconPath: "/images/logo/ethereum-rounded.svg",
      messageServiceAddress: getAddress(process.env.NEXT_PUBLIC_SEPOLIA_L1_MESSAGE_SERVICE ?? ""),
      tokenBridgeAddress: getAddress(process.env.NEXT_PUBLIC_SEPOLIA_L1_TOKEN_BRIDGE ?? ""),
      gasLimitSurplus: process.env.NEXT_PUBLIC_SEPOLIA_DEFAULT_GAS_LIMIT_SURPLUS
        ? BigInt(process.env.NEXT_PUBLIC_SEPOLIA_DEFAULT_GAS_LIMIT_SURPLUS)
        : BigInt(6000),
      profitMargin: process.env.NEXT_PUBLIC_SEPOLIA_PROFIT_MARGIN
        ? BigInt(process.env.NEXT_PUBLIC_SEPOLIA_PROFIT_MARGIN)
        : BigInt(1),
      cctpDomain: 0,
    },
    59141: {
      iconPath: "/images/logo/linea-sepolia.svg",
      messageServiceAddress: getAddress(process.env.NEXT_PUBLIC_SEPOLIA_LINEA_MESSAGE_SERVICE ?? ""),
      tokenBridgeAddress: getAddress(process.env.NEXT_PUBLIC_SEPOLIA_LINEA_TOKEN_BRIDGE ?? ""),
      gasLimitSurplus: process.env.NEXT_PUBLIC_SEPOLIA_DEFAULT_GAS_LIMIT_SURPLUS
        ? BigInt(process.env.NEXT_PUBLIC_SEPOLIA_DEFAULT_GAS_LIMIT_SURPLUS)
        : BigInt(6000),
      profitMargin: process.env.NEXT_PUBLIC_SEPOLIA_PROFIT_MARGIN
        ? BigInt(process.env.NEXT_PUBLIC_SEPOLIA_PROFIT_MARGIN)
        : BigInt(1),
      cctpDomain: 11,
    },
  },
  walletConnectId: process.env.NEXT_PUBLIC_WALLET_CONNECT_ID ?? "",
  storage: {
    // The storage will be cleared if its version is smaller than the one configured
    minVersion: process.env.NEXT_PUBLIC_STORAGE_MIN_VERSION ? parseInt(process.env.NEXT_PUBLIC_STORAGE_MIN_VERSION) : 1,
  },
  isCCTPEnabled: process.env.NEXT_PUBLIC_IS_CCTP_ENABLED === "true",
};

export async function getConfiguration(): Promise<Config> {
  return config;
}

// Schema validation
getConfiguration().then((config) => {
  configSchema.parse(config);
});
