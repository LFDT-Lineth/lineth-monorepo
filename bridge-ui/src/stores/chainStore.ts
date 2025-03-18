import { create } from "zustand";
import { createSelectorHooks, ZustandHookSelectors } from "auto-zustand-selectors-hook";
import { linea, lineaSepolia, mainnet, sepolia } from "viem/chains";
import { Chain } from "@/types";
import { generateChain, generateChains } from "@/utils";

export type ChainState = {
  chains: Chain[];
  fromChain: Chain;
  toChain: Chain;
};

export type ChainActions = {
  switchChain: () => void;
  setFromChain: (chain: Chain | undefined) => void;
  setToChain: (chain: Chain | undefined) => void;
};

export type ChainStore = ChainState & ChainActions;

export const defaultInitState: ChainState = {
  chains: generateChains([mainnet, sepolia, linea, lineaSepolia]),
  fromChain: generateChain(mainnet),
  toChain: generateChain(linea),
};

const useChainStoreBase = create<ChainStore>()((set, get) => ({
  ...defaultInitState,
  switchChain: () => {
    const { fromChain, toChain } = get();
    const tempFromChain = fromChain;
    set({ fromChain: toChain, toChain: tempFromChain });
  },
  setFromChain: (chain) => set({ fromChain: chain }),
  setToChain: (chain) => set({ toChain: chain }),
}));

export const useChainStore = createSelectorHooks(useChainStoreBase) as unknown as typeof useChainStoreBase &
  ZustandHookSelectors<ChainStore>;
