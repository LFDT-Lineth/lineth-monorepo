"use client";

import { type ReactNode, createContext, useState, useContext } from "react";

import { useStore } from "zustand";

import { isUndefined } from "@/utils/misc";

import { TokenState, type TokenStore, createTokenStore } from "./tokenStore";

export type TokenStoreApi = ReturnType<typeof createTokenStore>;

export const TokenStoreContext = createContext<TokenStoreApi | undefined>(undefined);

export interface TokenStoreProviderProps {
  children: ReactNode;
  initialState?: TokenState;
}

export function TokenStoreProvider({ children, initialState }: TokenStoreProviderProps) {
  const [store] = useState(() => createTokenStore(initialState));

  return <TokenStoreContext.Provider value={store}>{children}</TokenStoreContext.Provider>;
}

export const useTokenStore = <T,>(selector: (store: TokenStore) => T): T => {
  const tokenStoreContext = useContext(TokenStoreContext);

  if (isUndefined(tokenStoreContext)) {
    throw new Error(`useTokenStore must be used within TokenStoreProvider`);
  }

  return useStore(tokenStoreContext, selector);
};
