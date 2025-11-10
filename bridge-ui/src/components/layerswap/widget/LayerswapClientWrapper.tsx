"use client";

import { Swap, LayerswapProvider, LayerSwapSettings, ThemeData } from "@layerswap/widget";
import { EVMProvider } from "@layerswap/wallet-evm";
import useEVM from "./useCustomEvm";
import { config } from "@/config";

interface LayerswapClientWrapperProps {
  settings: LayerSwapSettings | undefined;
}

export function LayerswapClientWrapper({ settings }: LayerswapClientWrapperProps) {
  const evmProvider = {
    ...EVMProvider,
    walletConnectionProvider: useEVM,
  };

  return (
    <LayerswapProvider
      config={{
        settings,
        apiKey: config.layerswapApiKey,
        version: "mainnet",
        theme: themeData,
        initialValues: {
          defaultTab: "cex",
          to: "LINEA_MAINNET",
          lockTo: true,
        },
      }}
      walletProviders={[evmProvider]}
    >
      <Swap />
    </LayerswapProvider>
  );
}

const themeData: ThemeData = {
  buttonTextColor: "248, 247, 241",
  borderRadius: "large",
  header: {
    hideTabs: true,
    hideWallets: true,
  },
  cardBackgroundStyle: {
    backgroundColor: "255, 255, 255",
  },
  warning: {
    Foreground: "247, 213, 131",
    Background: "255, 241, 201",
  },
  error: {
    Foreground: "255, 97, 97",
    Background: "46, 27, 27",
  },
  success: {
    Foreground: "89, 224, 125",
    Background: "14, 43, 22",
  },
  primary: {
    DEFAULT: "97, 25, 239",
    "100": "202, 178, 250",
    "200": "176, 139, 247",
    "300": "149, 101, 244",
    "400": "123, 63, 242",
    "500": "97, 25, 239",
    "600": "74, 14, 194",
    "700": "54, 10, 142",
    "800": "34, 6, 89",
    "900": "14, 3, 37",
    text: "18, 18, 18",
  },
  secondary: {
    DEFAULT: "248, 247, 241",
    "100": "255, 255, 255",
    "200": "220, 219, 214",
    "300": "220, 219, 214",
    "400": "220, 219, 214",
    "500": "240, 240, 235",
    "600": "240, 240, 235",
    "700": "255, 255, 255",
    "800": "255, 255, 255",
    "900": "255, 255, 255",
    text: "18, 18, 18",
  },
};
