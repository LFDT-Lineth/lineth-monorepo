"use client";

import { useAccount } from "wagmi";
import { MdWarning } from "react-icons/md";
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
      {isConnected && token?.type === TokenType.USDC && (
        <div className="mb-4 min-w-min max-w-lg rounded-lg bg-warning p-2 text-warning-content shadow-lg">
          <div className="flex flex-col items-center justify-center gap-2 text-center">
            <MdWarning className="text-lg" />
            <p>The Linea Sepolia (Testnet) USDC bridge is being upgraded.</p>
            <p>
              To bridge USDC between Linea and Ethereum, you can use alternative bridge providers. Linea Mainnet is not
              effected
            </p>
          </div>
        </div>
      )}
      <FormProvider {...methods}>
        <Bridge />
      </FormProvider>
    </>
  );
}
