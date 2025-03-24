"use client";

import { useDynamicContext } from "@dynamic-labs/sdk-react-core";
import { ChainId, LiFiWidget, WidgetSkeleton, type WidgetConfig } from "@/lib/lifi";
import { ClientOnly } from "../client-only";
import atypTextFont from "@/assets/fonts/atypText";
import { CHAINS_RPC_URLS } from "@/constants";
import { config } from "@/config";

const widgetConfig: Partial<WidgetConfig> = {
  variant: "compact",
  subvariant: "default",
  appearance: "light",
  integrator: "Linea",
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
      body1: {
        fontSize: "0.875rem",
      },
      body2: {
        fontSize: "0.875rem",
      },
    },
    container: {
      borderRadius: "0.625rem",
      maxHeight: "80vh",
      maxWidth: "29.25rem",
      minWidth: "none",
      fontSize: "0.875rem",
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
          },
        },
      },
      MuiAppBar: {
        styleOverrides: {
          root: {
            display: "flex",
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
            fontSize: "0.875rem",
          },
        },
        defaultProps: {
          variant: "elevation",
        },
      },
      MuiInputCard: "",
    },
  },
  hiddenUI: ["appearance", "language"],
  sdkConfig: {
    rpcUrls: {
      [ChainId.ETH]: [CHAINS_RPC_URLS[ChainId.ETH]],
      [ChainId.LNA]: [CHAINS_RPC_URLS[ChainId.LNA]],
      [ChainId.ARB]: [CHAINS_RPC_URLS[ChainId.ARB]],
      [ChainId.AVA]: [CHAINS_RPC_URLS[ChainId.AVA]],
      [ChainId.BAS]: [CHAINS_RPC_URLS[ChainId.BAS]],
      [ChainId.BLS]: [CHAINS_RPC_URLS[ChainId.BLS]],
      [ChainId.BSC]: [CHAINS_RPC_URLS[ChainId.BSC]],
      [ChainId.CEL]: [CHAINS_RPC_URLS[ChainId.CEL]],
      [ChainId.MNT]: [CHAINS_RPC_URLS[ChainId.MNT]],
      [ChainId.OPT]: [CHAINS_RPC_URLS[ChainId.OPT]],
      [ChainId.POL]: [CHAINS_RPC_URLS[ChainId.POL]],
      [ChainId.SCL]: [CHAINS_RPC_URLS[ChainId.SCL]],
      [ChainId.ERA]: [CHAINS_RPC_URLS[ChainId.ERA]],
    },
  },
  chains: {
    deny: [
      ChainId.BTC,
      ChainId.SOL,
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
  bridges: {
    allow: ["stargateV2", "stargateV2Bus", "across", "hop", "squid", "relay"],
  },
  apiKey: config.lifiApiKey,
};

export function Widget() {
  const { setShowAuthFlow } = useDynamicContext();

  return (
    <div>
      <ClientOnly fallback={<WidgetSkeleton config={widgetConfig} />}>
        <LiFiWidget
          config={{
            ...widgetConfig,
            walletConfig: {
              onConnect() {
                setShowAuthFlow(true);
              },
            },
          }}
          integrator="linea"
        />
      </ClientOnly>
    </div>
  );
}
