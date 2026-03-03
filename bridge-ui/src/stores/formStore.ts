import { Address } from "viem";
import { createWithEqualityFn } from "zustand/traditional";
import { shallow } from "zustand/vanilla/shallow";

import { type AdapterModeId, Token, ClaimType } from "@/types";

import { defaultTokensConfig } from "./tokenStore";

export type FormState = {
  token: Token;
  recipient: Address;
  amount: bigint | null;
  balance: bigint;
  claim: ClaimType;
  gasFees: bigint;
  selectedMode: AdapterModeId | null;
};

export type FormActions = {
  setToken: (token: Token) => void;
  setRecipient: (recipient: Address) => void;
  setAmount: (amount: bigint) => void;
  setBalance: (balance: bigint) => void;
  setClaim: (claim: ClaimType) => void;
  setGasFees: (gasFees: bigint) => void;
  setSelectedMode: (mode: AdapterModeId | null) => void;
  resetForm(): void;
};

export type FormStore = FormState & FormActions;

export const defaultInitState: FormState = {
  token: defaultTokensConfig.MAINNET[0],
  amount: 0n,
  balance: 0n,
  recipient: "0x",
  claim: ClaimType.AUTO_SPONSORED,
  gasFees: 0n,
  selectedMode: null,
};

export const createFormStore = (defaultValues?: FormState) =>
  createWithEqualityFn<FormStore>((set) => {
    return {
      ...defaultInitState,
      ...defaultValues,
      setToken: (token) => set({ token }),
      setRecipient: (recipient) => set({ recipient }),
      setAmount: (amount) => set({ amount }),
      setBalance: (balance) => set({ balance }),
      setClaim: (claim) => set({ claim }),
      setGasFees: (gasFees) => set({ gasFees }),
      setSelectedMode: (selectedMode) => set({ selectedMode }),
      resetForm: () => set(defaultInitState),
    };
  }, shallow);
