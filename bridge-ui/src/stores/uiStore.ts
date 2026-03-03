import { create } from "zustand";

interface UiState {
  hideNoFeesPill: boolean;
  setHideNoFeesPill: (hide: boolean) => void;
}

export const useUiStore = create<UiState>((set) => ({
  hideNoFeesPill: false,
  setHideNoFeesPill: (hide) => set({ hideNoFeesPill: hide }),
}));
