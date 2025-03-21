import Link from "next/link";
import Modal from "@/components/modal";
import styles from "./transaction-confirmed.module.scss";
import { useNativeBridgeNavigationStore } from "@/stores";
import Button from "@/components/ui/button";
import { useQueryClient } from "@tanstack/react-query";

type Props = {
  isModalOpen: boolean;
  transactionType?: string;
  onCloseModal: () => void;
};

export default function TransactionConfirmed({ isModalOpen, transactionType, onCloseModal }: Props) {
  const setIsTransactionHistoryOpen = useNativeBridgeNavigationStore.useSetIsTransactionHistoryOpen();
  const queryClient = useQueryClient();

  return (
    <Modal title="Transaction confirmed!" isOpen={isModalOpen} onClose={onCloseModal} size="lg">
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          {transactionType === "approve"
            ? "You have successfully approved the token. You can now bridge your token."
            : "You may now bridge another transaction, check your transaction history, or stay ahead of the curve with the latest trending tokens."}
        </p>
        <div className={styles["list-button"]}>
          <Link
            className={styles["primary-btn"]}
            href="https://linea.build/ecosystem"
            target="_blank"
            rel="noopenner noreferrer"
          >
            See What&apos;s Trending
          </Link>

          <Button
            variant="link"
            className={styles["secondary-btn"]}
            onClick={() => {
              if (transactionType !== "approve") {
                setIsTransactionHistoryOpen(true);
                queryClient.invalidateQueries({ queryKey: ["transactionHistory"], exact: false });
              }
              onCloseModal();
            }}
          >
            {transactionType === "approve" ? "Bridge your token" : "View transactions"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
