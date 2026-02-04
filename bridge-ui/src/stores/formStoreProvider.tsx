"use client";

import { type ReactNode, createContext, useState, useContext } from "react";

import { useStore } from "zustand";

import { isUndefined } from "@/utils/misc";

import { FormState, type FormStore, createFormStore } from "./formStore";

export type FormStoreApi = ReturnType<typeof createFormStore>;

export const FormStoreContext = createContext<FormStoreApi | undefined>(undefined);

export interface FormStoreProviderProps {
  children: ReactNode;
  initialState?: FormState;
}

export function FormStoreProvider({ children, initialState }: FormStoreProviderProps) {
  const [store] = useState(() => createFormStore(initialState));

  return <FormStoreContext.Provider value={store}>{children}</FormStoreContext.Provider>;
}

export const useFormStore = <T,>(selector: (store: FormStore) => T): T => {
  const formStoreContext = useContext(FormStoreContext);

  if (isUndefined(formStoreContext)) {
    throw new Error(`useFormStore must be used within TokenStoreProvider`);
  }

  return useStore(formStoreContext, selector);
};
