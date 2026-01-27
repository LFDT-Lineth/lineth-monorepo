import { useState } from "react";

import TransactionDetails from "@/components/bridge/transaction-history/modal/transaction-details";
import { BridgeTransaction } from "@/types";

import Transaction from "./item";
import styles from "./list-transaction.module.scss";

type Props = {
  transactions: BridgeTransaction[];
};

export default function ListTransaction({ transactions }: Props) {
  const [currentTransaction, setCurrentTransaction] = useState<BridgeTransaction | undefined>(undefined);
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
      <ul className={styles["list"]} data-testid="native-bridge-transaction-history-list">
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
