"use client";

import { useEffect, useMemo } from "react";
import TransactionItem from "./TransactionItem";
import { TransactionHistory } from "@/models/history";
import { formatDate, fromUnixTime } from "date-fns";
import { NoTransactions } from "./NoTransaction";
import { useFetchHistory } from "@/hooks";
import RefreshHistoryButton from "./RefreshHistoryButton";

const groupByDay = (transactions: TransactionHistory[]): Record<string, TransactionHistory[]> => {
  return transactions.reduce(
    (acc, transaction) => {
      const date = formatDate(fromUnixTime(Number(transaction.timestamp)), "yyyy-MM-dd");
      if (!acc[date]) {
        acc[date] = [];
      }
      acc[date].push(transaction);
      return acc;
    },
    {} as Record<string, TransactionHistory[]>,
  );
};

function SkeletonLoader() {
  return (
    <div className="flex flex-col gap-8 bg-cardBg p-4">
      {Array.from({ length: 3 }).map((_, groupIndex) => (
        <div key={groupIndex} className="flex flex-col gap-4">
          <div className="skeleton h-6 w-1/3 bg-backgroundColor"></div>
          {Array.from({ length: 2 }).map((_, itemIndex) => (
            <div
              key={itemIndex}
              className="grid grid-cols-1 items-center gap-0 rounded-lg bg-backgroundColor p-4 sm:grid-cols-1 md:grid-cols-6 md:gap-4"
            >
              <div className="grid grid-cols-2 gap-4 border-b border-cardBg py-4 md:col-span-2 md:border-none md:p-0">
                <div className="skeleton h-4 w-1/2 bg-cardBg"></div>
                <div className="skeleton h-4 w-1/2 bg-cardBg"></div>
              </div>
              <div className="hidden px-6 md:col-span-2 md:block md:border-x md:border-card">
                <div className="skeleton h-4 w-full bg-cardBg"></div>
              </div>
              <div className="grid grid-cols-2 items-center gap-4 border-b border-cardBg py-4 md:col-span-2 md:border-none md:p-0">
                <div className="skeleton h-4 w-1/2 bg-cardBg"></div>
                <div className="skeleton h-4 w-1/2 bg-cardBg"></div>
              </div>
              <div className="px-6 pt-4 md:hidden md:pt-0">
                <div className="skeleton h-4 w-full bg-cardBg"></div>
              </div>
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

function TransactionGroup({ date, transactions }: { date: string; transactions: TransactionHistory[] }) {
  return (
    <div className="flex flex-col gap-2">
      <span className="block text-base-content">{formatDate(date, "PPP")}</span>
      {transactions.map((transaction, transactionIndex) => {
        if (transaction.message) {
          const { message, ...bridgingTransaction } = transaction;
          return (
            <TransactionItem
              key={`transaction-group-${date}-item-${transactionIndex}`}
              transaction={bridgingTransaction}
              message={message}
            />
          );
        }
      })}
    </div>
  );
}

export function Transactions() {
  // Context
  const { transactions, fetchHistory, isLoading } = useFetchHistory();

  useEffect(() => {
    fetchHistory();
  }, [fetchHistory]);

  const groupedTransactions = useMemo(() => groupByDay(transactions), [transactions]);

  if (isLoading && transactions.length === 0) {
    return <SkeletonLoader />;
  }

  if (transactions.length === 0) {
    return <NoTransactions />;
  }

  return (
    <div className="flex flex-col gap-8 rounded-lg bg-cardBg p-4">
      <RefreshHistoryButton fetchHistory={fetchHistory} isLoading={isLoading} />
      {Object.keys(groupedTransactions).map((date) => (
        <TransactionGroup key={`transaction-group-${date}`} date={date} transactions={groupedTransactions[date]} />
      ))}
    </div>
  );
}
