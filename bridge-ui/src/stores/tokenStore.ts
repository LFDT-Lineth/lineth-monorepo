import { createWithEqualityFn } from "zustand/traditional";
import { shallow } from "zustand/vanilla/shallow";

import { BridgeProvider, NetworkTokens, Token } from "@/types";

export const defaultTokensConfig: NetworkTokens = {
  MAINNET: [
    {
      type: ["eth"],
      name: "Ether",
      symbol: "ETH",
      decimals: 18,
      L1: "0x0000000000000000000000000000000000000000",
      L2: "0x0000000000000000000000000000000000000000",
      image: "https://s2.coinmarketcap.com/static/img/coins/64x64/1027.png",
      isDefault: true,
      bridgeProvider: BridgeProvider.NATIVE,
    },
  ],
  SEPOLIA: [
    {
      type: ["eth"],
      name: "Ether",
      symbol: "ETH",
      decimals: 18,
      L1: "0x0000000000000000000000000000000000000000",
      L2: "0x0000000000000000000000000000000000000000",
      image: "https://s2.coinmarketcap.com/static/img/coins/64x64/1027.png",
      isDefault: true,
      bridgeProvider: BridgeProvider.NATIVE,
    },
  ],
};

export type TokenState = {
  tokensList: NetworkTokens;
  selectedToken: Token;
};

export type TokenActions = {
  setSelectedToken: (token: Token) => void;
};

export type TokenStore = TokenState & TokenActions;

export const defaultInitState: TokenState = {
  tokensList: defaultTokensConfig,
  selectedToken: defaultTokensConfig.MAINNET[0],
};

export const createTokenStore = (initState: TokenState = defaultInitState) => {
  return createWithEqualityFn<TokenStore>()(
    (set) => ({
      ...initState,
      setSelectedToken: (token: Token) => {
        set({ selectedToken: token });
      },
    }),
    shallow,
  );
};
