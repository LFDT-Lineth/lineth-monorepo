import { useState } from "react";
import Image from "next/image";
import TokenModal from "@/components/bridge/modal/token-modal";
import Button from "@/components/ui/button";
import CaretDownIcon from "@/assets/icons/caret-down.svg";
import styles from "./token-list.module.scss";
import { useFormStore } from "@/stores";

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
      <TokenModal isModalOpen={isModalOpen} onCloseModal={closeModal} />
    </div>
  );
}
