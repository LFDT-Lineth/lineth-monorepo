"use client";

import { useAccount } from "wagmi";
import Bridge from "../bridge/Bridge";
import { BridgeExternal } from "./BridgeExternal";
import { useTokenStore } from "@/stores/tokenStoreProvider";
import { FormProvider, useForm } from "react-hook-form";
import { BridgeForm } from "@/models";
import { useChainStore } from "@/stores/chainStore";
import { TokenType } from "@/config";

export default function BridgeLayout() {
  const { isConnected } = useAccount();

  const configContextValue = useTokenStore((state) => state.tokensList);
  const token = useChainStore((state) => state.token);

  const methods = useForm<BridgeForm>({
    defaultValues: {
      token: configContextValue?.UNKNOWN[0],
      claim: token?.type === TokenType.ETH ? "auto" : "manual",
      amount: "",
      minFees: 0n,
      gasFees: 0n,
      bridgingAllowed: false,
      balance: "0",
    },
  });

  return (
    <>
      {!isConnected && (
        <div className="mb-4 min-w-min max-w-lg rounded-lg bg-cardBg p-2 shadow-lg">
          <BridgeExternal />
        </div>
      )}
      <FormProvider {...methods}>
        <Bridge />
      </FormProvider>
    </>
  );
}
