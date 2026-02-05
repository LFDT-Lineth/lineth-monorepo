"use client";

import { ChainId, WidgetConfig, WidgetSkeleton } from "@lifi/widget";
import { useWeb3AuthConnect } from "@web3auth/modal/react";
import dynamic from "next/dynamic";

import atypTextFont from "@/assets/fonts/atypText";
import { config as appConfig } from "@/config";
import { CHAINS_RPC_URLS } from "@/constants/chains";
import { ETH_SYMBOL } from "@/constants/tokens";

const widgetConfig: Partial<WidgetConfig> = {
  variant: "compact",
  subvariant: "default",
  appearance: "light",
  toChain: ChainId.LNA,
  toToken: ETH_SYMBOL,
  theme: {
    palette: {
      primary: {
        main: "#6119ef",
      },
      secondary: {
        main: "#6119ef",
      },
      background: {
        default: "#ffffff",
        paper: "#f8f7f2",
      },
      text: {
        primary: "#121212",
        secondary: "#525252",
      },
      grey: {
        200: "#f5f5f5",
        300: "#f1f1f1",
        700: "#525252",
        800: "#222222",
      },
    },
    shape: {
      borderRadius: 10,
      borderRadiusSecondary: 30,
      borderRadiusTertiary: 24,
    },
    typography: {
      fontFamily: atypTextFont.style.fontFamily,
      fontWeightBold: 700,
      fontWeightMedium: 500,
      fontWeightRegular: 400,
      body1: {
        fontSize: "0.875rem !important",
        fontWeight: "500 !important",
      },
    },
    container: {
      borderRadius: "0.625rem",
      maxHeight: "none",
      maxWidth: "100%",
      minWidth: "min(416px, calc(100% - 3rem))",
      fontSize: "0.875rem",
      filter: "none",
    },
    components: {
      MuiButton: {
        styleOverrides: {
          root: {
            fontSize: "0.875rem",
          },
        },
      },
      MuiIconButton: {
        styleOverrides: {
          root: {
            fontSize: "0.875rem",
            padding: "0.5rem",
          },
        },
      },
      MuiAppBar: {
        styleOverrides: {
          root: {
            display: "flex",
            minHeight: "2.5rem",
            fontSize: "0.875rem",
            justifyContent: "flex-end",
            ["p"]: {
              visibility: "hidden",
              fontSize: "0.875rem",
            },
            ["p:before"]: {
              content: '""',
              visibility: "visible",
            },
          },
        },
      },
      MuiCard: {
        styleOverrides: {
          root: {
            filter: "none !important",
            WebkitFilter: "none !important",
            fontSize: "0.875rem !important",
            ":hover": {
              boxShadow: "inset 0 0 0 0.125rem var(--color-icterine)",
            },
          },
        },
        defaultProps: {
          variant: "elevation",
          sx: {
            ".MuiCardContent-root": {
              ":hover": {
                boxShadow: "inset 0 0 0 0.125rem var(--color-icterine)",
              },
            },
          },
        },
      },
    },
  },
  hiddenUI: ["appearance", "language"],
  sdkConfig: {
    rpcUrls: {
      [ChainId.SOL]: CHAINS_RPC_URLS[ChainId.SOL],
      [ChainId.ETH]: CHAINS_RPC_URLS[ChainId.ETH],
      [ChainId.LNA]: CHAINS_RPC_URLS[ChainId.LNA],
      [ChainId.ARB]: CHAINS_RPC_URLS[ChainId.ARB],
      [ChainId.AVA]: CHAINS_RPC_URLS[ChainId.AVA],
      [ChainId.BAS]: CHAINS_RPC_URLS[ChainId.BAS],
      [ChainId.BLS]: CHAINS_RPC_URLS[ChainId.BLS],
      [ChainId.BSC]: CHAINS_RPC_URLS[ChainId.BSC],
      [ChainId.CEL]: CHAINS_RPC_URLS[ChainId.CEL],
      [ChainId.MNT]: CHAINS_RPC_URLS[ChainId.MNT],
      [ChainId.OPT]: CHAINS_RPC_URLS[ChainId.OPT],
      [ChainId.POL]: CHAINS_RPC_URLS[ChainId.POL],
      [ChainId.SCL]: CHAINS_RPC_URLS[ChainId.SCL],
      [ChainId.ERA]: CHAINS_RPC_URLS[ChainId.ERA],
    },
  },
  chains: {
    deny: [
      ChainId.BTC,
      ChainId.PZE,
      ChainId.MOR,
      ChainId.FUS,
      ChainId.BOB,
      ChainId.MAM,
      ChainId.LSK,
      ChainId.UNI,
      ChainId.IMX,
      ChainId.GRA,
      ChainId.TAI,
      ChainId.SOE,
      ChainId.FRA,
      ChainId.ABS,
      ChainId.RSK,
      ChainId.WCC,
      ChainId.BER,
      ChainId.KAI,
    ],
  },
  apiKey: appConfig.lifiApiKey,
};

const LiFiWidget = dynamic(() => import("@lifi/widget").then((mod) => mod.LiFiWidget), {
  ssr: false,
  loading: () => <WidgetSkeleton config={widgetConfig} />,
});

export function Widget() {
  const { connect } = useWeb3AuthConnect();

  return (
    <LiFiWidget
      config={{
        ...widgetConfig,
        walletConfig: {
          onConnect: connect,
        },
      }}
      integrator={appConfig.lifiIntegrator}
    />
  );
}
