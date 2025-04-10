import { watchAccount } from "@wagmi/core";
import { useEffect } from "react";
import { useChainStore } from "@/stores";
import { config } from "@/lib/wagmi";
import useChains from "./useChains";
import { Chain } from "@/types";
import { isUndefined } from "@/utils";

const useInitialiseChain = () => {
  const chains = useChains();
  const setFromChain = useChainStore.useSetFromChain();
  const setToChain = useChainStore.useSetToChain();

  useEffect(() => {
    const unwatch = watchAccount(config, {
      onChange(account) {
        const chain = chains.find((chain: Chain) => chain.id === account?.chain?.id);

        if (isUndefined(chain)) {
          return;
        }

        setFromChain(chain);

        if (chain.testnet) {
          setToChain(chains.find((c: Chain) => c.testnet && c.layer !== chain.layer));
        } else {
          setToChain(chains.find((c: Chain) => !c.testnet && c.layer !== chain.layer));
        }
      },
    });

    return () => {
      unwatch();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chains]);
};

export default useInitialiseChain;
