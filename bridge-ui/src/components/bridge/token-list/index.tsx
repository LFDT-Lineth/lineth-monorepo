import { useState } from "react";

import dynamic from "next/dynamic";
import Image from "next/image";

import CaretDownIcon from "@/assets/icons/caret-down.svg";
import Button from "@/components/ui/button";
import { useFormStore } from "@/stores";

import styles from "./token-list.module.scss";

const TokenModal = dynamic(() => import("@/components/bridge/modal/token-modal"), {
  ssr: false,
});

export default function TokenList() {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const token = useFormStore((state) => state.token);

  const openModal = () => setIsModalOpen(true);
  const closeModal = () => setIsModalOpen(false);

  return (
    <div className={styles["wrapper"]}>
      {token && (
        <Button
          className={styles["token-select-btn"]}
          onClick={openModal}
          data-testid="native-bridge-open-token-list-modal"
        >
          <Image src={token.image} alt={token.name} width={24} height={24} />
          {token.symbol}
          <CaretDownIcon className={styles["arrow-down-icon"]} />
        </Button>
      )}
      {isModalOpen && <TokenModal isModalOpen={isModalOpen} onCloseModal={closeModal} />}
    </div>
  );
}
