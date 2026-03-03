"use client";

import { useConnection } from "wagmi";

import { useTokens } from "@/hooks";
import { useChainStore } from "@/stores/chainStore";
import type { FormState } from "@/stores/formStore";
import { FormStoreProvider } from "@/stores/formStoreProvider";
import { ChainLayer, ClaimType } from "@/types";

export default function Layout({ children }: { children: React.ReactNode }) {
  const { address } = useConnection();
  const tokens = useTokens();
  const fromChain = useChainStore.useFromChain();

  const initialFormState: FormState = {
    token: tokens[0],
    claim: fromChain?.layer === ChainLayer.L1 ? ClaimType.AUTO_SPONSORED : ClaimType.MANUAL,
    amount: null,
    gasFees: 0n,
    balance: 0n,
    recipient: address || "0x",
    selectedMode: null,
  };

  return <FormStoreProvider initialState={initialFormState}>{children}</FormStoreProvider>;
}
