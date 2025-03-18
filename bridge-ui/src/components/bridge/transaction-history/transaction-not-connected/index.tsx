import styles from "./transaction-not-connected.module.scss";
import TransactionCircleIcon from "@/assets/icons/transaction-circle.svg";
import ConnectButton from "@/components/connect-button";

export default function TransactionNotConnected() {
  return (
    <div className={styles["content"]}>
      <span className={styles["icon"]}>
        <TransactionCircleIcon />
      </span>
      <p className={styles["title"]}>Please connect your wallet.</p>
      <ConnectButton text="Connect" />
    </div>
  );
}
