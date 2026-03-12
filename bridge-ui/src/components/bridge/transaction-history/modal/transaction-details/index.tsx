import { useMemo } from "react";

import Link from "next/link";
import { formatEther } from "viem";
import { useTransactionReceipt } from "wagmi";

import ArrowRightIcon from "@/assets/icons/arrow-right.svg";
import Modal from "@/components/modal";
import { useClaimingTx } from "@/hooks";
import { buildExplorerUrl } from "@/lib/urls";
import { BridgeTransaction, TransactionStatus } from "@/types";
import { formatBalance, formatHex, formatTimestamp } from "@/utils/format";

import ClaimActions from "./claim-actions";
import styles from "./transaction-details.module.scss";

type Props = {
  transaction: BridgeTransaction | undefined;
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function TransactionDetails({ transaction, isModalOpen, onCloseModal }: Props) {
  const formattedDate = transaction?.timestamp ? formatTimestamp(Number(transaction.timestamp), "MMM, dd, yyyy") : "";
  const formattedTime = transaction?.timestamp ? formatTimestamp(Number(transaction.timestamp), "ppp") : "";

  const claimingTx = useClaimingTx(transaction);

  const displayClaimingTx = claimingTx || transaction?.claimingTx;

  const { data: initialTransactionReceipt } = useTransactionReceipt({
    hash: transaction?.bridgingTx as `0x${string}`,
    chainId: transaction?.fromChain.id,
    query: {
      enabled: !!transaction?.bridgingTx && transaction?.status === TransactionStatus.COMPLETED,
      staleTime: 1000 * 60 * 5,
      refetchOnMount: false,
    },
  });

  const { data: claimingTransactionReceipt } = useTransactionReceipt({
    hash: displayClaimingTx as `0x${string}`,
    chainId: transaction?.toChain.id,
    query: {
      enabled: !!displayClaimingTx && transaction?.status === TransactionStatus.COMPLETED,
      staleTime: 1000 * 60 * 5,
      refetchOnMount: false,
    },
  });

  const gasFees = useMemo(() => {
    const initialTransactionFee =
      initialTransactionReceipt &&
      "gasUsed" in initialTransactionReceipt &&
      "effectiveGasPrice" in initialTransactionReceipt &&
      initialTransactionReceipt.gasUsed &&
      initialTransactionReceipt.effectiveGasPrice
        ? (initialTransactionReceipt.gasUsed as bigint) * (initialTransactionReceipt.effectiveGasPrice as bigint)
        : 0n;

    const claimingTransactionFee =
      claimingTransactionReceipt &&
      "gasUsed" in claimingTransactionReceipt &&
      "effectiveGasPrice" in claimingTransactionReceipt &&
      claimingTransactionReceipt.gasUsed &&
      claimingTransactionReceipt.effectiveGasPrice
        ? (claimingTransactionReceipt.gasUsed as bigint) * (claimingTransactionReceipt.effectiveGasPrice as bigint)
        : 0n;

    return initialTransactionFee + claimingTransactionFee;
  }, [initialTransactionReceipt, claimingTransactionReceipt]);

  const bridgingTxUrl = transaction?.bridgingTx
    ? buildExplorerUrl(transaction.fromChain.blockExplorers?.default.url, "tx", transaction.bridgingTx)
    : undefined;
  const claimingTxUrl = displayClaimingTx
    ? buildExplorerUrl(transaction?.toChain.blockExplorers?.default.url, "tx", displayClaimingTx)
    : undefined;

  return (
    <Modal title="Transaction details" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <ul className={styles.list}>
          <li>
            <span>Timestamp</span>
            <div className={styles["date-time"]}>
              <span>{formattedDate}</span>
              <span>{formattedTime}</span>
            </div>
          </li>
          <li>
            <span>{transaction?.fromChain.name} Tx hash</span>
            <div className={styles.hash}>
              {bridgingTxUrl ? (
                <Link href={bridgingTxUrl} target="_blank" rel="noopener noreferrer">
                  {formatHex(transaction?.bridgingTx)}
                </Link>
              ) : (
                <span>{formatHex(transaction?.bridgingTx)}</span>
              )}
              <ArrowRightIcon />
            </div>
          </li>
          <li>
            <span>{transaction?.toChain.name} Tx hash</span>
            <div className={styles.hash}>
              {displayClaimingTx ? (
                claimingTxUrl ? (
                  <Link href={claimingTxUrl} target="_blank" rel="noopener noreferrer">
                    {formatHex(displayClaimingTx)}
                  </Link>
                ) : (
                  <span>{formatHex(displayClaimingTx)}</span>
                )
              ) : (
                <span>Pending</span>
              )}

              <ArrowRightIcon />
            </div>
          </li>
          {transaction?.status === TransactionStatus.COMPLETED && (
            <li>
              <span>Gas fee</span>
              <span className={styles.price}>{formatBalance(formatEther(gasFees), 8)} ETH</span>
            </li>
          )}
        </ul>
        {transaction?.status === TransactionStatus.READY_TO_CLAIM && (
          <ClaimActions transaction={transaction} onCloseModal={onCloseModal} />
        )}
      </div>
    </Modal>
  );
}
