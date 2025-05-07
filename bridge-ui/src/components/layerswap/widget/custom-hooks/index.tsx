"use client";
import { WalletHooksProvider } from "@layerswap/widget";
import useCustomEVM from "./useCustomEvm";
import { ReactNode } from "react";

export default function CustomHooks({ children }: { children: ReactNode }) {
  const customEvm = useCustomEVM();
  return <WalletHooksProvider overides={{ evm: customEvm }}>{children}</WalletHooksProvider>;
}
