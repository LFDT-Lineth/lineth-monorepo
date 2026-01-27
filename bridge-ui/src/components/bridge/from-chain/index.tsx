import { useState } from "react";

import dynamic from "next/dynamic";
import Image from "next/image";

import { useChains } from "@/hooks";
import { useChainStore } from "@/stores";
import { Chain } from "@/types";

import styles from "./from-chain.module.scss";

const SelectNetwork = dynamic(() => import("@/components/bridge/modal/select-network"), {
  ssr: false,
});

export default function FromChain() {
  const [isModalOpen, setIsModalOpen] = useState(false);

  const chains = useChains();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const setFromChain = useChainStore.useSetFromChain();
  const setToChain = useChainStore.useSetToChain();

  const openModal = () => setIsModalOpen(true);
  const closeModal = () => {
    setIsModalOpen(false);
  };

  const handleSelectNetwork = (chain: Chain) => {
    if (chain.id === toChain?.id) {
      setToChain(fromChain);
      setFromChain(chain);
      return;
    }

    setFromChain(chain);
    setToChain(chains.find((c: Chain) => c.id === chain.toChainId));
  };

  return (
    <>
      <button onClick={openModal} className={styles["from"]} type="button">
        <div className={styles["name"]}>From</div>
        <div className={styles["info"]}>
          {fromChain?.iconPath && (
            <Image src={fromChain.iconPath} width="40" height="40" alt={fromChain.nativeCurrency.symbol} />
          )}
          <div className={styles["info-value"]}>{fromChain.name}</div>
        </div>
      </button>
      {isModalOpen && (
        <SelectNetwork
          isModalOpen={isModalOpen}
          onCloseModal={closeModal}
          onClick={handleSelectNetwork}
          networks={chains}
        />
      )}
    </>
  );
}
