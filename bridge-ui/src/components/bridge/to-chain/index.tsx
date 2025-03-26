import Image from "next/image";
import styles from "./to-chain.module.scss";
import SelectNetwork from "@/components/bridge/modal/select-network";
import { useState } from "react";
import { useChainStore } from "@/stores";
import { useChains } from "@/hooks";
import { Chain } from "@/types";

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

    if (chain.testnet) {
      setFromChain(chains.find((c: Chain) => c.testnet && c.layer !== chain.layer));
    } else {
      setFromChain(chains.find((c: Chain) => !c.testnet && c.layer !== chain.layer));
    }
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
      <SelectNetwork
        isModalOpen={isModalOpen}
        onCloseModal={closeModal}
        networks={chains}
        onClick={handleSelectNetwork}
      />
    </>
  );
}
