import Modal from "@/components/modal";
import Button from "@/components/ui/button";
import { useNativeBridgeNavigationStore } from "@/stores/nativeBridgeNavigationStore";

import styles from "./manual-claim.module.scss";

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function ManualClaim({ isModalOpen, onCloseModal }: Props) {
  const setIsTransactionHistoryOpen = useNativeBridgeNavigationStore.useSetIsTransactionHistoryOpen();

  return (
    <Modal title="Manual claim on destination" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          You will need to claim your transaction on the destination chain with an additional transaction that requires
          ETH on the destination chain. This can be done on the{" "}
          <Button
            variant="link"
            onClick={() => {
              setIsTransactionHistoryOpen(true);
              onCloseModal();
            }}
          >
            Transaction page
          </Button>
          .
        </p>
        <Button fullWidth onClick={onCloseModal}>
          OK
        </Button>
      </div>
    </Modal>
  );
}
