import Modal from "@/components/modal";
import styles from "./estimated-time.module.scss";
import Button from "@/components/ui/button";

type Props = {
  type: "deposit" | "withdraw";
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function EstimatedTime({ type, isModalOpen, onCloseModal }: Props) {
  const estimatedTime = type === "deposit" ? "20 minutes" : "8 to 32 hours";
  const estimatedTimeType = type === "deposit" ? "deposits" : "withdrawals";

  return (
    <Modal title="Estimated time" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          Linea has an approximate {estimatedTime} delay on {estimatedTimeType} as a security measure.
        </p>
        <Button fullWidth onClick={onCloseModal}>
          OK
        </Button>
      </div>
    </Modal>
  );
}
