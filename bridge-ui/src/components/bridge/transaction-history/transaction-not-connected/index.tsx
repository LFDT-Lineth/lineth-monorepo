import TransactionCircleIcon from "@/assets/icons/transaction-circle.svg";
import ConnectButton from "@/components/connect-button";

import styles from "./transaction-not-connected.module.scss";

export default function TransactionNotConnected() {
  return (
    <div className={styles["content"]}>
      <span className={styles["icon"]}>
        <TransactionCircleIcon />
      </span>
      <p data-testid="tx-history-connect-your-wallet-text" className={styles["title"]}>
        Please connect your wallet.
      </p>
      <ConnectButton text="Connect" />
    </div>
  );
}
