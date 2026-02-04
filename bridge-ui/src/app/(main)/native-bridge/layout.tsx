"use client";

import { useConnection } from "wagmi";

import { useTokens } from "@/hooks";
import { useChainStore } from "@/stores/chainStore";
import type { FormState } from "@/stores/formStore";
import { FormStoreProvider } from "@/stores/formStoreProvider";
import { CCTPMode, ChainLayer, ClaimType } from "@/types";

export default function Layout({ children }: { children: React.ReactNode }) {
  const { address } = useConnection();
  const tokens = useTokens();
  const fromChain = useChainStore.useFromChain();

  const initialFormState: FormState = {
    token: tokens[0],
    claim: fromChain?.layer === ChainLayer.L1 ? ClaimType.AUTO_SPONSORED : ClaimType.MANUAL,
    amount: null,
    minimumFees: 0n,
    gasFees: 0n,
    bridgingFees: 0n,
    balance: 0n,
    recipient: address || "0x",
    cctpMode: CCTPMode.STANDARD,
  };

  return <FormStoreProvider initialState={initialFormState}>{children}</FormStoreProvider>;
}
