"use client";

import { type ReactNode, createContext, useRef, useContext } from "react";

import { useStore } from "zustand";

import { isUndefined } from "@/utils";

import { FormState, type FormStore, createFormStore } from "./formStore";

export type FormStoreApi = ReturnType<typeof createFormStore>;

export const FormStoreContext = createContext<FormStoreApi | undefined>(undefined);

export interface FormStoreProviderProps {
  children: ReactNode;
  initialState?: FormState;
}

export function FormStoreProvider({ children, initialState }: FormStoreProviderProps) {
  const storeRef = useRef<FormStoreApi | undefined>(undefined);
  if (isUndefined(storeRef.current)) {
    storeRef.current = createFormStore(initialState);
  }

  return <FormStoreContext.Provider value={storeRef.current}>{children}</FormStoreContext.Provider>;
}

export const useFormStore = <T,>(selector: (store: FormStore) => T): T => {
  const formStoreContext = useContext(FormStoreContext);

  if (isUndefined(formStoreContext)) {
    throw new Error(`useFormStore must be used within TokenStoreProvider`);
  }

  return useStore(formStoreContext, selector);
};
