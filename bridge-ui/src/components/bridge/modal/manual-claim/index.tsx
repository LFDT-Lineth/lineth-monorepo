import Modal from "@/components/modal";

import styles from "./manual-claim.module.scss";
import Link from "next/link";
import Button from "@/components/ui/button";

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function ManualClaim({ isModalOpen, onCloseModal }: Props) {
  return (
    <Modal title="Manual claim on destination" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          You will need to claim your transaction on the destination chain with an additional transaction that requires
          ETH on the destination chain. This can be done on the <Link href="/transactions">Transaction page</Link>.
        </p>
        <Button fullWidth onClick={onCloseModal}>
          OK
        </Button>
      </div>
    </Modal>
  );
}
