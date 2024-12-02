"use client";

import ConnectButton from "@/components/ConnectButton";
import { Transactions } from "@/components/transactions";
import { useAccount } from "wagmi";

export default function TransactionsPage() {
  const { isConnected } = useAccount();

  if (!isConnected) {
    return (
      <div className="m-auto min-w-min max-w-5xl">
        <div className="flex min-h-80 flex-col items-center justify-center gap-8 rounded-lg bg-cardBg p-4">
          <span>Please connect your wallet.</span>
          <ConnectButton />
        </div>
      </div>
    );
  }

  return (
    <div className="m-auto min-w-min max-w-5xl">
      <h1 className="mb-6 text-4xl md:hidden">Transactions</h1>
      <Transactions />
    </div>
  );
}
