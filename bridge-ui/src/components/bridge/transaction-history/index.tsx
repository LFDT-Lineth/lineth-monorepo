"use client";

import { useAccount } from "wagmi";
import styles from "./transaction-history.module.scss";
import ArrowLeftIcon from "@/assets/icons/arrow-left.svg";
import { useNativeBridgeNavigationStore } from "@/stores";
import Button from "../../ui/button";
import ListTransaction from "./list-transaction";
import NoTransaction from "./no-transaction";
import TransactionNotConnected from "./transaction-not-connected";
import { useTransactionHistory } from "@/hooks";
import SkeletonLoader from "./skeleton-loader";

export default function TransactionHistory() {
  const { isConnected } = useAccount();
  const setIsTransactionHistoryOpen = useNativeBridgeNavigationStore.useSetIsTransactionHistoryOpen();
  const setIsBridgeOpen = useNativeBridgeNavigationStore.useSetIsBridgeOpen();
  const { transactions, isLoading } = useTransactionHistory();

  if (isLoading) {
    return (
      <div className={styles["transaction-history-wrapper"]}>
        <div className={styles.headline}>
          <div className={styles["action"]}>
            <Button
              className={styles["go-back-button"]}
              variant="link"
              onClick={() => {
                setIsTransactionHistoryOpen(false);
                setIsBridgeOpen(true);
              }}
            >
              <ArrowLeftIcon className={styles["go-back-icon"]} />
            </Button>
          </div>
          <h2 className={styles.title}>Transaction History</h2>
        </div>
        <SkeletonLoader />
      </div>
    );
  }

  return (
    <div className={styles["transaction-history-wrapper"]}>
      <div className={styles.headline}>
        <div className={styles["action"]}>
          <Button
            className={styles["go-back-button"]}
            variant="link"
            onClick={() => {
              setIsTransactionHistoryOpen(false);
              setIsBridgeOpen(true);
            }}
          >
            <ArrowLeftIcon className={styles["go-back-icon"]} />
          </Button>
        </div>
        <h2 className={styles.title}>Transaction History</h2>
      </div>

      {isConnected ? (
        transactions?.length ? (
          <ListTransaction transactions={transactions} />
        ) : (
          <NoTransaction />
        )
      ) : (
        <TransactionNotConnected />
      )}
    </div>
  );
}
