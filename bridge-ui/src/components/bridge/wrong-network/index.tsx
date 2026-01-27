import TransactionCircleIcon from "@/assets/icons/transaction-circle.svg";

import styles from "./wrong-network.module.scss";

export default function WrongNetwork() {
  return (
    <div className={styles["wrong-network-wrapper"]}>
      <div className={styles["content"]}>
        <span className={styles["icon"]}>
          <TransactionCircleIcon />
        </span>
        <p className={styles["title"]}>Please switch network.</p>
        <p className={styles["description"]}>
          This bridge doesn&apos;t work with Solana. Please switch to the Ethereum or Linea network.
        </p>
      </div>
    </div>
  );
}
