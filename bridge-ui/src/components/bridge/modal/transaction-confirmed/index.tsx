import { useQueryClient } from "@tanstack/react-query";

import Modal from "@/components/modal";
import Button from "@/components/ui/button";
import { USDC_SYMBOL } from "@/constants";
import { useFormStore, useNativeBridgeNavigationStore } from "@/stores";

import styles from "./transaction-confirmed.module.scss";

type Props = {
  isModalOpen: boolean;
  transactionType?: string;
  onCloseModal: () => void;
};

export default function TransactionConfirmed({ isModalOpen, transactionType, onCloseModal }: Props) {
  const setIsTransactionHistoryOpen = useNativeBridgeNavigationStore.useSetIsTransactionHistoryOpen();
  const token = useFormStore((state) => state.token);
  const queryClient = useQueryClient();

  const getMessage = () => {
    if (transactionType === "approve") {
      return "You have successfully approved the token. You can now bridge your token.";
    }
    if (token.symbol === USDC_SYMBOL) {
      return "Your transaction is confirmed on the source chain. Check your transaction history to claim your tokens once they become available on the destination chain.";
    }
    return "You may now bridge another transaction, check your transaction history, or stay ahead of the curve with the latest trending tokens.";
  };

  return (
    <Modal title="Transaction confirmed!" isOpen={isModalOpen} onClose={onCloseModal} size="lg">
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>{getMessage()}</p>
        <div className={styles["list-button"]}>
          <Button
            className={styles["primary-btn"]}
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
