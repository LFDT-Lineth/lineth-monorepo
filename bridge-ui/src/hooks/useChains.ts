import { useMemo } from "react";

import { useChainStore } from "@/stores/chainStore";
import { useConfigStore } from "@/stores/configStore";

const useChains = () => {
  const chains = useChainStore.useChains();
  const testnetsEnabled = useConfigStore.useShowTestnet();

  return useMemo(() => {
    if (!testnetsEnabled) {
      return chains.filter((chain) => !chain.testnet);
    }

    return chains.filter((chain) => chain.testnet);
  }, [chains, testnetsEnabled]);
};

export default useChains;
