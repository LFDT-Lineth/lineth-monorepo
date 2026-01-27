import { useEffect } from "react";

import { watchAccount } from "@wagmi/core";
import { useConfig } from "wagmi";

import { useChainStore } from "@/stores";
import { Chain } from "@/types";
import { isUndefined } from "@/utils";

import useChains from "./useChains";

const useInitialiseChain = () => {
  const wagmiConfig = useConfig();
  const chains = useChains();
  const setFromChain = useChainStore.useSetFromChain();
  const setToChain = useChainStore.useSetToChain();

  useEffect(() => {
    const unwatch = watchAccount(wagmiConfig, {
      onChange(account) {
        const chain = chains.find((chain: Chain) => chain.id === account?.chain?.id);

        if (isUndefined(chain)) {
          return;
        }

        setFromChain(chain);
        setToChain(chains.find((c: Chain) => c.id === chain.toChainId));
      },
    });

    return () => {
      unwatch();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chains]);
};

export default useInitialiseChain;
