import { useMemo } from "react";
import { useChainStore, useTokenStore } from "@/stores";
import { ChainLayer, Token } from "@/types";
import { config } from "@/config";

const useTokens = (): Token[] => {
  const tokensList = useTokenStore((state) => state.tokensList);
  const fromChain = useChainStore.useFromChain();

  return useMemo(() => {
    if (!fromChain) return [];

    if (fromChain.testnet) {
      if (fromChain.layer !== ChainLayer.L2) {
        return tokensList.SEPOLIA.filter(
          (token) => !token.type.includes("native") && (config.isCCTPEnabled || token.symbol !== "USDC"),
        );
      }
      return config.isCCTPEnabled ? tokensList.SEPOLIA : tokensList.SEPOLIA.filter((token) => token.symbol !== "USDC");
    }

    if (fromChain.layer !== ChainLayer.L2) {
      return tokensList.MAINNET.filter(
        (token) => !token.type.includes("native") && (config.isCCTPEnabled || token.symbol !== "USDC"),
      );
    }

    return config.isCCTPEnabled ? tokensList.MAINNET : tokensList.MAINNET.filter((token) => token.symbol !== "USDC");
  }, [fromChain, tokensList.MAINNET, tokensList.SEPOLIA]);
};

export default useTokens;
