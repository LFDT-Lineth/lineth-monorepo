import { getAddress, zeroAddress } from "viem";
import { configSchema, Config } from "./config.schema";

export const config: Config = {
  chains: {
    1: {
      iconPath: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/ethereum-rounded.svg`,
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
      iconPath: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/linea-rounded.svg`,
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
      iconPath: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/ethereum-rounded.svg`,
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
      iconPath: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/linea-sepolia.svg`,
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
    // Local networks for testing purposes
    ...(process.env.NEXT_PUBLIC_E2E_TEST_MODE === "true"
      ? {
          31648428: {
            iconPath: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/ethereum-rounded.svg`,
            messageServiceAddress: "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9",
            tokenBridgeAddress: "0x2279B7A0a67DB372996a5FaB50D91eAA73d2eBe6",
            gasLimitSurplus: 6000n,
            profitMargin: 2n,
            cctpDomain: 0,
            cctpTokenMessengerV2Address: zeroAddress, // Not used in E2E tests
            cctpMessageTransmitterV2Address: zeroAddress, // Not used in E2E tests
          },
          1337: {
            iconPath: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/linea-sepolia.svg`,
            messageServiceAddress: "0xe537D669CA013d86EBeF1D64e40fC74CADC91987",
            tokenBridgeAddress: "0x5C95Bcd50E6D1B4E3CDC478484C9030Ff0a7D493",
            gasLimitSurplus: 6000n,
            profitMargin: 2n,
            cctpDomain: 0,
            cctpTokenMessengerV2Address: zeroAddress, // Not used in E2E tests
            cctpMessageTransmitterV2Address: zeroAddress, // Not used in E2E tests
          },
        }
      : {}),
  },
  walletConnectId: process.env.NEXT_PUBLIC_WALLET_CONNECT_ID ?? "",
  storage: {
    // The storage will be cleared if its version is smaller than the one configured
    minVersion: process.env.NEXT_PUBLIC_STORAGE_MIN_VERSION ? parseInt(process.env.NEXT_PUBLIC_STORAGE_MIN_VERSION) : 1,
  },
  isCctpEnabled: process.env.NEXT_PUBLIC_IS_CCTP_ENABLED === "true",
  infuraApiKey: process.env.NEXT_PUBLIC_INFURA_ID ?? "",
  alchemyApiKey: process.env.NEXT_PUBLIC_ALCHEMY_API_KEY ?? "",
  quickNodeApiKey: process.env.NEXT_PUBLIC_QUICKNODE_ID ?? "",
  web3AuthClientId: process.env.NEXT_PUBLIC_WEB3_AUTH_CLIENT_ID ?? "",
  lifiApiKey: process.env.NEXT_PUBLIC_LIFI_API_KEY ?? "",
  lifiIntegrator: process.env.NEXT_PUBLIC_LIFI_INTEGRATOR_NAME ?? "",
  onRamperApiKey: process.env.NEXT_PUBLIC_ONRAMPER_API_KEY ?? "",
  layerswapApiKey: process.env.NEXT_PUBLIC_LAYERSWAP_API_KEY ?? "",
  tokenListUrls: {
    mainnet: process.env.NEXT_PUBLIC_MAINNET_TOKEN_LIST ?? "",
    sepolia: process.env.NEXT_PUBLIC_SEPOLIA_TOKEN_LIST ?? "",
  },
  e2eTestMode: process.env.NEXT_PUBLIC_E2E_TEST_MODE === "true",
};

export async function getConfiguration(): Promise<Config> {
  return config;
}

// Schema validation
getConfiguration().then((config) => {
  configSchema.parse(config);
});
