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
      cctpTokenMessengerV2Address: getAddress("0x28b5a0e9C621a5BadaA536219b3a228C8168cf5d"),
      cctpMessageTransmitterV2Address: getAddress("0x81D40F21F12A8F0E3252Bccb954D722d4c464B64"),
    },
    59144: {
      iconPath: "/images/logo/linea-rounded.svg",
      messageServiceAddress: getAddress(process.env.NEXT_PUBLIC_MAINNET_LINEA_MESSAGE_SERVICE ?? ""),
      tokenBridgeAddress: getAddress(process.env.NEXT_PUBLIC_MAINNET_LINEA_TOKEN_BRIDGE ?? ""),
      gasLimitSurplus: process.env.NEXT_PUBLIC_MAINNET_DEFAULT_GAS_LIMIT_SURPLUS
        ? BigInt(process.env.NEXT_PUBLIC_MAINNET_DEFAULT_GAS_LIMIT_SURPLUS)
        : BigInt(6000),
      profitMargin: process.env.NEXT_PUBLIC_MAINNET_PROFIT_MARGIN
        ? BigInt(process.env.NEXT_PUBLIC_MAINNET_PROFIT_MARGIN)
        : BigInt(1),
      cctpDomain: 11,
      cctpTokenMessengerV2Address: getAddress("0x28b5a0e9C621a5BadaA536219b3a228C8168cf5d"),
      cctpMessageTransmitterV2Address: getAddress("0x81D40F21F12A8F0E3252Bccb954D722d4c464B64"),
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
      cctpTokenMessengerV2Address: getAddress("0x8FE6B999Dc680CcFDD5Bf7EB0974218be2542DAA"),
      cctpMessageTransmitterV2Address: getAddress("0xE737e5cEBEEBa77EFE34D4aa090756590b1CE275"),
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
      cctpTokenMessengerV2Address: getAddress("0x8FE6B999Dc680CcFDD5Bf7EB0974218be2542DAA"),
      cctpMessageTransmitterV2Address: getAddress("0xE737e5cEBEEBa77EFE34D4aa090756590b1CE275"),
    },
  },
  walletConnectId: process.env.NEXT_PUBLIC_WALLET_CONNECT_ID ?? "",
  storage: {
    // The storage will be cleared if its version is smaller than the one configured
    minVersion: process.env.NEXT_PUBLIC_STORAGE_MIN_VERSION ? parseInt(process.env.NEXT_PUBLIC_STORAGE_MIN_VERSION) : 1,
  },
  isCctpEnabled: process.env.NEXT_PUBLIC_IS_CCTP_ENABLED === "true",
  infuraApiKey: process.env.NEXT_PUBLIC_INFURA_ID ?? "",
  dynamicEnvironmentId: process.env.NEXT_PUBLIC_DYNAMIC_ENVIRONMENT_ID ?? "",
  lifiApiKey: process.env.NEXT_PUBLIC_LIFI_API_KEY ?? "",
  onRamperApiKey: process.env.NEXT_PUBLIC_ONRAMPER_API_KEY ?? "",
  tokenListUrls: {
    mainnet: process.env.NEXT_PUBLIC_MAINNET_TOKEN_LIST ?? "",
    sepolia: process.env.NEXT_PUBLIC_SEPOLIA_TOKEN_LIST ?? "",
  },
};

export async function getConfiguration(): Promise<Config> {
  return config;
}

// Schema validation
getConfiguration().then((config) => {
  configSchema.parse(config);
});
