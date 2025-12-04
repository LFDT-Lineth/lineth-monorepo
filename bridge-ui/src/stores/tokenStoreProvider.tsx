"use client";

import { type ReactNode, createContext, useRef, useContext } from "react";
import { useStore } from "zustand";
import { TokenState, type TokenStore, createTokenStore } from "./tokenStore";
import { isUndefined } from "@/utils";

export type TokenStoreApi = ReturnType<typeof createTokenStore>;

export const TokenStoreContext = createContext<TokenStoreApi | undefined>(undefined);

export interface TokenStoreProviderProps {
  children: ReactNode;
  initialState?: TokenState;
}

export function TokenStoreProvider({ children, initialState }: TokenStoreProviderProps) {
  const storeRef = useRef<TokenStoreApi | undefined>(undefined);
  if (isUndefined(storeRef.current)) {
    storeRef.current = createTokenStore(initialState);
  }

  return <TokenStoreContext.Provider value={storeRef.current}>{children}</TokenStoreContext.Provider>;
}

export const useTokenStore = <T,>(selector: (store: TokenStore) => T): T => {
  const tokenStoreContext = useContext(TokenStoreContext);

  if (isUndefined(tokenStoreContext)) {
    throw new Error(`useTokenStore must be used within TokenStoreProvider`);
  }

  return useStore(tokenStoreContext, selector);
};
