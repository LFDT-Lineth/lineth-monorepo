import { useMemo } from "react";

import { useChainStore } from "@/stores/chainStore";
import { useTokenStore } from "@/stores/tokenStoreProvider";
import { Token } from "@/types";
import { isUndefined } from "@/utils/misc";

const useTokens = (): Token[] => {
  const tokensList = useTokenStore((state) => state.tokensList);
  const fromChain = useChainStore.useFromChain();

  return useMemo(() => {
    if (isUndefined(fromChain)) return [];

    return fromChain.testnet ? tokensList.SEPOLIA : tokensList.MAINNET;
  }, [fromChain, tokensList.MAINNET, tokensList.SEPOLIA]);
};

export default useTokens;
