import { create } from "zustand";
import { createSelectorHooks, ZustandHookSelectors } from "auto-zustand-selectors-hook";
import { config } from "@/config";

import { createJSONStorage, persist } from "zustand/middleware";
import { VisitedModalType } from "@/components/modal/first-time-visit";

export type SupportedCurrencies = "usd" | "eur";

export type CurrencyOption = {
  value: SupportedCurrencies;
  label: string;
  flag: string;
};

export type ConfigState = {
  agreeToTerms: boolean;
  rehydrated: boolean;
  supportedCurrencies: CurrencyOption[];
  currency: CurrencyOption;
  showTestnet: boolean;
  visitedModal: Record<VisitedModalType, boolean>;
};

export type ConfigActions = {
  setAgreeToTerms: (agree: boolean) => void;
  setCurrency: (currency: CurrencyOption) => void;
  setShowTestnet: (show: boolean) => void;
  setVisitedModal: (modal: VisitedModalType) => void;
};

export type ConfigStore = ConfigState & ConfigActions;

export const defaultInitState: ConfigState = {
  agreeToTerms: false,
  rehydrated: false,
  supportedCurrencies: [
    { value: "usd", label: "USD", flag: "🇺🇸" },
    { value: "eur", label: "EUR", flag: "🇪🇺" },
  ],
  currency: {
    value: "usd",
    label: "USD",
    flag: "🇺🇸",
  },
  showTestnet: false,
  visitedModal: {
    "all-bridges": false,
    "native-bridge": false,
    buy: false,
  },
};

const useConfigStoreBase = create<ConfigStore>()(
  persist(
    (set) => ({
      ...defaultInitState,
      setAgreeToTerms: (agree) => set({ agreeToTerms: agree }),
      setCurrency: (currency: CurrencyOption) => set({ currency }),
      setShowTestnet: (show: boolean) => set({ showTestnet: show }),
      setVisitedModal: (modal) =>
        set((state) => ({
          visitedModal: {
            ...state.visitedModal,
            [modal]: true,
          },
        })),
    }),
    {
      name: "config-storage",
      version: config.storage.minVersion,
      storage: createJSONStorage(() => localStorage),
      migrate: () => {
        return defaultInitState;
      },
      onRehydrateStorage: () => (state) => {
        if (state) {
          state.rehydrated = true;
        }
      },
    },
  ),
);

export const useConfigStore = createSelectorHooks(useConfigStoreBase) as unknown as typeof useConfigStoreBase &
  ZustandHookSelectors<ConfigStore>;
