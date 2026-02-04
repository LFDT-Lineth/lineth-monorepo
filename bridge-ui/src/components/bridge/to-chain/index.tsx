import { useState } from "react";

import dynamic from "next/dynamic";
import Image from "next/image";

import { useChains } from "@/hooks";
import { useChainStore } from "@/stores/chainStore";
import { Chain } from "@/types";

import styles from "./to-chain.module.scss";

const SelectNetwork = dynamic(() => import("@/components/bridge/modal/select-network"), {
  ssr: false,
});

export default function ToChain() {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const chains = useChains();

  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const setFromChain = useChainStore.useSetFromChain();
  const setToChain = useChainStore.useSetToChain();

  const openModal = () => setIsModalOpen(true);
  const closeModal = () => setIsModalOpen(false);

  const handleSelectNetwork = (chain: Chain) => {
    if (chain.id === fromChain?.id) {
      setFromChain(toChain);
      setToChain(chain);
      return;
    }
    setToChain(chain);
    setFromChain(chains.find((c: Chain) => c.id === chain.toChainId));
  };

  return (
    <>
      <button onClick={openModal} className={styles["to"]} type="button">
        <div className={styles["name"]}>To</div>
        <div className={styles["info"]}>
          {toChain?.iconPath && (
            <Image src={toChain.iconPath} width="40" height="40" alt={toChain.nativeCurrency.symbol} />
          )}
          <div className={styles["info-value"]}>{toChain?.name}</div>
        </div>
      </button>
      {isModalOpen && (
        <SelectNetwork
          isModalOpen={isModalOpen}
          onCloseModal={closeModal}
          networks={chains}
          onClick={handleSelectNetwork}
        />
      )}
    </>
  );
}
