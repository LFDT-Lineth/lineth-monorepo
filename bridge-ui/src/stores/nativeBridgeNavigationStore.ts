import { create } from "zustand";
import { createSelectorHooks, ZustandHookSelectors } from "auto-zustand-selectors-hook";

export type NativeBridgeNavigationState = {
  isBridgeOpen: boolean;
  isTransactionHistoryOpen: boolean;
};

export type NativeBridgeNavigationActions = {
  setIsBridgeOpen: (opened: boolean) => void;
  setIsTransactionHistoryOpen: (opened: boolean) => void;
};

export type NativeBridgeNavigationStore = NativeBridgeNavigationState & NativeBridgeNavigationActions;

export const defaultInitState: NativeBridgeNavigationState = {
  isBridgeOpen: true,
  isTransactionHistoryOpen: false,
};

const useNativeBridgeNavigationStoreBase = create<NativeBridgeNavigationStore>()((set) => ({
  ...defaultInitState,
  setIsBridgeOpen: (opened) => set({ isBridgeOpen: opened }),
  setIsTransactionHistoryOpen: (opened) => set({ isTransactionHistoryOpen: opened }),
}));

export const useNativeBridgeNavigationStore = createSelectorHooks(
  useNativeBridgeNavigationStoreBase,
) as unknown as typeof useNativeBridgeNavigationStoreBase & ZustandHookSelectors<NativeBridgeNavigationStore>;
