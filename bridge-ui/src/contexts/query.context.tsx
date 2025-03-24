"use client";

import { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

type Web3ProviderProps = {
  children: ReactNode;
};

const queryClient = new QueryClient();

export function QueryProvider({ children }: Web3ProviderProps) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
