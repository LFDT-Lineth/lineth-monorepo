import { useMemo } from "react";

import { config } from "@/config";
import { useChainStore } from "@/stores/chainStore";
import { useConfigStore } from "@/stores/configStore";

const useChains = () => {
  const chains = useChainStore.useChains();
  const testnetsEnabled = useConfigStore.useShowTestnet();

  return useMemo(() => {
    if (config.e2eTestMode) {
      return chains.filter((chain) => chain.localNetwork);
    }

    if (!testnetsEnabled) {
      return chains.filter((chain) => !chain.testnet);
    }

    return chains.filter((chain) => chain.testnet);
  }, [chains, testnetsEnabled]);
};

export default useChains;
