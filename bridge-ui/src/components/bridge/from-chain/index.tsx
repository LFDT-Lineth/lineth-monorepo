import Image from "next/image";
import styles from "./from-chain.module.scss";
import SelectNetwork from "@/components/bridge/modal/select-network";
import { useState } from "react";
import { useChainStore } from "@/stores";
import { useChains } from "@/hooks";
import { Chain } from "@/types";

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

    if (chain.testnet) {
      setToChain(chains.find((c: Chain) => c.testnet && c.layer !== chain.layer));
    } else {
      setToChain(chains.find((c: Chain) => !c.testnet && c.layer !== chain.layer));
    }
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
      <SelectNetwork
        isModalOpen={isModalOpen}
        onCloseModal={closeModal}
        onClick={handleSelectNetwork}
        networks={chains}
      />
    </>
  );
}
