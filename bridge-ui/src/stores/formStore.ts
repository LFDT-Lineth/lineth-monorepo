import { Address } from "viem";
import { defaultTokensConfig } from "./tokenStore";
import { createWithEqualityFn } from "zustand/traditional";
import { shallow } from "zustand/vanilla/shallow";
import { Token } from "@/types";

export type FormState = {
  token: Token;
  recipient: Address;
  amount: bigint | null;
  balance: bigint;
  claim: "auto" | "manual";
  gasFees: bigint;
  bridgingFees: bigint;
  minimumFees: bigint;
};

export type FormActions = {
  setToken: (token: Token) => void;
  setRecipient: (recipient: Address) => void;
  setAmount: (amount: bigint) => void;
  setBalance: (balance: bigint) => void;
  setClaim: (claim: "auto" | "manual") => void;
  setGasFees: (gasFees: bigint) => void;
  setBridgingFees: (bridgingFees: bigint) => void;
  setMinimumFees: (minimumFees: bigint) => void;
  resetForm(): void;
};

export type FormStore = FormState & FormActions;

export const defaultInitState: FormState = {
  token: defaultTokensConfig.MAINNET[0],
  amount: 0n,
  balance: 0n,
  recipient: "0x",
  claim: "auto",
  gasFees: 0n,
  bridgingFees: 0n,
  minimumFees: 0n,
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
      setBridgingFees: (bridgingFees) => set({ bridgingFees }),
      setMinimumFees: (minimumFees) => set({ minimumFees }),
      resetForm: () => set(defaultInitState),
    };
  }, shallow);
