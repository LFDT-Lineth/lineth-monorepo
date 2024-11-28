import { createWithEqualityFn } from "zustand/traditional";
import { shallow } from "zustand/vanilla/shallow";
import { createJSONStorage, persist } from "zustand/middleware";
import { config, NetworkTokens, NetworkType, TokenInfo, TokenType } from "@/config";
import { getTokenConfig } from "@/services/tokenService";

export const defaultTokensConfig: NetworkTokens = {
  MAINNET: [
    {
      name: "Ether",
      symbol: "ETH",
      decimals: 18,
      type: TokenType.ETH,
      L1: null,
      L2: null,
      UNKNOWN: null,
      image: "https://s2.coinmarketcap.com/static/img/coins/64x64/1027.png",
      isDefault: true,
    },
  ],
  SEPOLIA: [
    {
      name: "Ether",
      symbol: "ETH",
      decimals: 18,
      type: TokenType.ETH,
      L1: null,
      L2: null,
      UNKNOWN: null,
      image: "https://s2.coinmarketcap.com/static/img/coins/64x64/1027.png",
      isDefault: true,
    },
  ],
  UNKNOWN: [
    {
      name: "Ether",
      symbol: "ETH",
      decimals: 18,
      type: TokenType.ETH,
      L1: null,
      L2: null,
      UNKNOWN: null,
      image: "https://s2.coinmarketcap.com/static/img/coins/64x64/1027.png",
      isDefault: true,
    },
  ],
};

export type TokenState = {
  tokensList: NetworkTokens;
};

export type TokenActions = {
  upsertToken: (token: TokenInfo, network: NetworkType) => void;
};

export type TokenStore = TokenState & TokenActions;

export const defaultInitState: TokenState = {
  tokensList: defaultTokensConfig,
};

export const createTokenStore = (initState: TokenState = defaultInitState) => {
  return createWithEqualityFn<TokenStore>()(
    persist(
      (set, get) => ({
        ...initState,
        upsertToken: (token: TokenInfo, network: NetworkType) => {
          const { tokensList } = get();
          if (network === NetworkType.WRONG_NETWORK) {
            return;
          }

          const networkTokens = tokensList[network];
          const existingTokenIndex = networkTokens.findIndex((t) => t.L1 === token.L1 || t.L2 === token.L2);

          let updatedTokens;
          if (existingTokenIndex !== -1) {
            updatedTokens = [...networkTokens];
            updatedTokens[existingTokenIndex] = token;
          } else {
            updatedTokens = [...networkTokens, token];
          }

          set({
            tokensList: {
              ...tokensList,
              [network]: updatedTokens,
            },
          });
        },
      }),
      {
        name: "token-storage", // name of the item in the storage (must be unique)
        storage: createJSONStorage(() => localStorage),
        version: parseInt(config.storage.minVersion),
        migrate: async () => {
          return getTokenConfig();
        },
      },
    ),
    shallow,
  );
};
