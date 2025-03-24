import { useState } from "react";
import styles from "./list-transaction.module.scss";
import Transaction from "./item";
import TransactionDetails from "@/components/bridge/transaction-history/modal/transaction-details";
import { BridgeTransaction } from "@/types";

type Props = {
  transactions: BridgeTransaction[];
};

export default function ListTransaction({ transactions }: Props) {
  const [currentTransaction, setCurrentTransaction] = useState<BridgeTransaction | undefined>(false || undefined);
  const handleCloseModal = () => {
    setCurrentTransaction(undefined);
  };
  const handleClickTransaction = (transactionHash: string) => {
    const transaction = transactions.find((t) => t.bridgingTx === transactionHash);
    if (transaction) {
      setCurrentTransaction(transaction);
    }
  };
  return (
    <>
      <ul className={styles["list"]}>
        {transactions.map((item, index) => (
          <Transaction key={`transaction-${item.bridgingTx}-${index}`} onClick={handleClickTransaction} {...item} />
        ))}
      </ul>
      <TransactionDetails
        transaction={currentTransaction}
        isModalOpen={!!currentTransaction}
        onCloseModal={handleCloseModal}
      />
    </>
  );
}
