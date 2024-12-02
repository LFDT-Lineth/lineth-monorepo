"use client";

import { useContext } from "react";
import { formatUnits } from "viem";
import { OnChainMessageStatus } from "@consensys/linea-sdk";
import { ModalContext } from "@/contexts/modal.context";
import StatusText from "./StatusText";
import TransactionProgressBar from "./TransactionProgressBar";
import TransactionDetailsModal from "./modals/TransactionDetailsModal";
import { NETWORK_ID_TO_NAME } from "@/utils/constants";
import { getChainNetworkLayerByChainId } from "@/utils/chainsUtil";
import { TransactionHistory } from "@/models/history";
import { MessageWithStatus } from "@/hooks";
import { cn } from "@/utils/cn";

export enum TransactionStatus {
  READY_TO_CLAIM = "READY_TO_CLAIM",
  PENDING = "PENDING",
  COMPLETED = "COMPLETED",
}

type TransactionItemProps = {
  transaction: TransactionHistory;
  message: MessageWithStatus;
};

function TransactionStatusSection({ status }: { status: OnChainMessageStatus }) {
  return (
    <div className="px-6 md:px-0">
      <div className="text-xs uppercase">Status</div>
      <StatusText status={status} />
    </div>
  );
}

function TransactionNetworkSection({ label, networkId }: { label: string; networkId: number }) {
  return (
    <div className="px-6 md:px-0">
      <div className="text-xs uppercase">{label}</div>
      <span>{NETWORK_ID_TO_NAME[networkId]}</span>
    </div>
  );
}

function TransactionAmountSection({ amount, decimals, symbol }: { amount: bigint; decimals: number; symbol: string }) {
  return (
    <div className="px-6 md:px-0">
      <div className="text-xs uppercase">Amount</div>
      <span className="font-semibold">
        {formatUnits(amount, decimals)} {symbol}
      </span>
    </div>
  );
}

export default function TransactionItem({ transaction, message }: TransactionItemProps) {
  const { handleShow, handleClose } = useContext(ModalContext);

  return (
    <div
      className={cn(
        `grid grid-cols-1 items-center gap-0 rounded-lg p-4 hover:cursor-pointer sm:grid-cols-1 md:grid-cols-6 md:gap-4`,
        {
          "bg-orange-light hover:opacity-80": message.status === OnChainMessageStatus.UNKNOWN,
          "bg-primary-light hover:opacity-80": message.status === OnChainMessageStatus.CLAIMABLE,
          "bg-secondary-light hover:opacity-80": message.status === OnChainMessageStatus.CLAIMED,
        },
      )}
      onClick={() => {
        handleShow(<TransactionDetailsModal transaction={transaction} message={message} handleClose={handleClose} />, {
          showCloseButton: true,
        });
      }}
    >
      <div className="grid grid-cols-2 gap-4 border-b border-card py-4 md:col-span-2 md:border-none md:p-0">
        <TransactionStatusSection status={message.status} />
        <TransactionNetworkSection label="From" networkId={transaction.fromChain.id} />
      </div>

      <div className="hidden px-6 md:col-span-2 md:block md:border-x md:border-card">
        <TransactionProgressBar
          status={message.status}
          transactionTimestamp={transaction.timestamp}
          fromChain={getChainNetworkLayerByChainId(transaction.fromChain.id)}
        />
      </div>

      <div className="grid grid-cols-2 items-center gap-4 border-b border-card py-4 md:col-span-2 md:border-none md:p-0">
        <TransactionNetworkSection label="To" networkId={transaction.toChain.id} />
        <TransactionAmountSection
          amount={transaction.amount}
          decimals={transaction.token.decimals}
          symbol={transaction.token.symbol}
        />
      </div>

      <div className="px-6 pt-4 md:hidden md:pt-0">
        <TransactionProgressBar
          status={message.status}
          transactionTimestamp={transaction.timestamp}
          fromChain={getChainNetworkLayerByChainId(transaction.fromChain.id)}
        />
      </div>
    </div>
  );
}
