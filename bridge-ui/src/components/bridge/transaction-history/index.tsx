"use client";

import { useConnection } from "wagmi";

import ArrowLeftIcon from "@/assets/icons/arrow-left.svg";
import { useTransactionHistory } from "@/hooks";
import { useNativeBridgeNavigationStore } from "@/stores/nativeBridgeNavigationStore";

import ListTransaction from "./list-transaction";
import NoTransaction from "./no-transaction";
import SkeletonLoader from "./skeleton-loader";
import styles from "./transaction-history.module.scss";
import TransactionNotConnected from "./transaction-not-connected";
import Button from "../../ui/button";

export default function TransactionHistory() {
  const { isConnected } = useConnection();
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
              data-testid="transaction-history-close-btn"
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
            data-testid="transaction-history-close-btn"
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
