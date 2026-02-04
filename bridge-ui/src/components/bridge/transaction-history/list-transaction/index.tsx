import { useState, useCallback, useMemo } from "react";

import TransactionDetails from "@/components/bridge/transaction-history/modal/transaction-details";
import { BridgeTransaction } from "@/types";

import Transaction from "./item";
import styles from "./list-transaction.module.scss";

type Props = {
  transactions: BridgeTransaction[];
};

export default function ListTransaction({ transactions }: Props) {
  const [currentTransaction, setCurrentTransaction] = useState<BridgeTransaction | undefined>(undefined);

  // Build index map for O(1) lookup instead of O(n) find
  const transactionsByHash = useMemo(() => new Map(transactions.map((t) => [t.bridgingTx, t])), [transactions]);

  const handleCloseModal = useCallback(() => {
    setCurrentTransaction(undefined);
  }, []);

  const handleClickTransaction = useCallback(
    (transactionHash: string) => {
      const transaction = transactionsByHash.get(transactionHash);
      if (transaction) {
        setCurrentTransaction(transaction);
      }
    },
    [transactionsByHash],
  );
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
