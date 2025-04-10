import { useMemo } from "react";
import { useChainStore, useTokenStore } from "@/stores";
import { ChainLayer, Token } from "@/types";
import { config } from "@/config";
import { USDC_SYMBOL } from "@/constants";
import { isUndefined } from "@/utils";

const useTokens = (): Token[] => {
  const tokensList = useTokenStore((state) => state.tokensList);
  const fromChain = useChainStore.useFromChain();

  return useMemo(() => {
    if (isUndefined(fromChain)) return [];

    if (fromChain.testnet) {
      if (fromChain.layer !== ChainLayer.L2) {
        return tokensList.SEPOLIA.filter(
          (token) => !token.type.includes("native") && (config.isCctpEnabled || token.symbol !== USDC_SYMBOL),
        );
      }
      return config.isCctpEnabled
        ? tokensList.SEPOLIA
        : tokensList.SEPOLIA.filter((token) => token.symbol !== USDC_SYMBOL);
    }

    if (fromChain.layer !== ChainLayer.L2) {
      return tokensList.MAINNET.filter(
        (token) => !token.type.includes("native") && (config.isCctpEnabled || token.symbol !== USDC_SYMBOL),
      );
    }

    return config.isCctpEnabled
      ? tokensList.MAINNET
      : tokensList.MAINNET.filter((token) => token.symbol !== USDC_SYMBOL);
  }, [fromChain, tokensList.MAINNET, tokensList.SEPOLIA]);
};

export default useTokens;
