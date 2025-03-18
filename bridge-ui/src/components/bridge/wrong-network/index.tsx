import styles from "./wrong-network.module.scss";
import TransactionCircleIcon from "@/assets/icons/transaction-circle.svg";

export default function WrongNetwork() {
  return (
    <div className={styles["content"]}>
      <span className={styles["icon"]}>
        <TransactionCircleIcon />
      </span>
      <p className={styles["title"]}>Please switch network.</p>
      <p className={styles["description"]}>
        The native bridge only supports the following networks: Ethereum, Sepolia, Linea Sepolia and Linea mainnet.
      </p>
    </div>
  );
}
