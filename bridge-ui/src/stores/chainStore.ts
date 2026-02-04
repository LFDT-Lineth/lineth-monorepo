import { createSelectorHooks, ZustandHookSelectors } from "auto-zustand-selectors-hook";
import { linea, lineaSepolia, mainnet, sepolia } from "viem/chains";
import { create } from "zustand";

import { config } from "@/config";
import { localL1Network, localL2Network } from "@/constants/chains";
import { Chain } from "@/types";
import { generateChain, generateChains } from "@/utils/chains";

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
  chains: generateChains([
    ...(config.e2eTestMode ? [localL1Network, localL2Network] : [mainnet, sepolia, linea, lineaSepolia]),
  ]),
  fromChain: generateChain(config.e2eTestMode ? localL1Network : mainnet),
  toChain: generateChain(config.e2eTestMode ? localL2Network : linea),
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
