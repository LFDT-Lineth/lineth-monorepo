import TransactionCircleIcon from "@/assets/icons/transaction-circle.svg";
import Button from "@/components/ui/button";
import { useNativeBridgeNavigationStore } from "@/stores";

import styles from "./no-transaction.module.scss";

export default function NoTransaction() {
  const setIsTransactionHistoryOpen = useNativeBridgeNavigationStore.useSetIsTransactionHistoryOpen();
  const setIsBridgeOpen = useNativeBridgeNavigationStore.useSetIsBridgeOpen();

  return (
    <div className={styles["content"]}>
      <span className={styles["icon"]}>
        <TransactionCircleIcon />
      </span>
      <p className={styles["title"]}>No transactions yet</p>
      <p className={styles["desc"]}>Use the bridge and view your transactions and their status here.</p>
      <Button
        className={styles["link"]}
        onClick={() => {
          setIsTransactionHistoryOpen(false);
          setIsBridgeOpen(true);
        }}
      >
        Bridge assets
      </Button>
    </div>
  );
}
