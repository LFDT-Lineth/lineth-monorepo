"use client";

import { useTokens } from "@/hooks";
import { useAccount } from "wagmi";
import { FormState, FormStoreProvider, useChainStore } from "@/stores";
import { ChainLayer } from "@/types";

export default function Layout({ children }: { children: React.ReactNode }) {
  const { address } = useAccount();
  const tokens = useTokens();
  const fromChain = useChainStore.useFromChain();

  const initialFormState: FormState = {
    token: tokens[0],
    claim: fromChain?.layer === ChainLayer.L1 ? "auto" : "manual",
    amount: null,
    minimumFees: 0n,
    gasFees: 0n,
    bridgingFees: 0n,
    balance: 0n,
    recipient: address || "0x",
  };

  return <FormStoreProvider initialState={initialFormState}>{children}</FormStoreProvider>;
}
