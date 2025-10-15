import { useEffect, useMemo } from "react";
import Link from "next/link";
import { useAccount, useSwitchChain, useTransactionReceipt } from "wagmi";
import { formatEther } from "viem";
import { useQueryClient } from "@tanstack/react-query";
import Modal from "@/components/modal";
import styles from "./transaction-details.module.scss";
import Button from "@/components/ui/button";
import ArrowRightIcon from "@/assets/icons/arrow-right.svg";
import { useClaim, useClaimingTx, useBridgeTransactionMessage } from "@/hooks";
import { BridgeTransaction, TransactionStatus } from "@/types";
import { formatBalance, formatHex, formatTimestamp } from "@/utils";

type Props = {
  transaction: BridgeTransaction | undefined;
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function TransactionDetails({ transaction, isModalOpen, onCloseModal }: Props) {
  const { chain } = useAccount();
  const { switchChain, isPending: isSwitchingChain } = useSwitchChain();

  const formattedDate = transaction?.timestamp ? formatTimestamp(Number(transaction.timestamp), "MMM, dd, yyyy") : "";
  const formattedTime = transaction?.timestamp ? formatTimestamp(Number(transaction.timestamp), "ppp") : "";

  // Hydrate BridgeTransaction.message with params required for claim tx
  const { message, isLoading: isLoadingClaimTxParams } = useBridgeTransactionMessage(transaction);
  if (transaction && message) transaction.message = message;

  // Hydrate BridgeTransaction.claimingTx
  const claimingTx = useClaimingTx(transaction);
  if (transaction && claimingTx && !transaction?.claimingTx) transaction.claimingTx = claimingTx;

  const { claim, isConfirming, isPending, isConfirmed } = useClaim({
    status: transaction?.status,
    type: transaction?.type,
    fromChain: transaction?.fromChain,
    toChain: transaction?.toChain,
    args: transaction?.message,
  });

  const queryClient = useQueryClient();
  useEffect(() => {
    if (isConfirmed) {
      queryClient.invalidateQueries({ queryKey: ["transactionHistory"], exact: false });
      onCloseModal();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isConfirmed]);

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
    hash: transaction?.claimingTx as `0x${string}`,
    chainId: transaction?.toChain.id,
    query: {
      enabled: !!transaction?.claimingTx && transaction?.status === TransactionStatus.COMPLETED,
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

  const buttonText = useMemo(() => {
    if (isLoadingClaimTxParams) {
      return "Loading Claim Data...";
    }

    if (isPending || isConfirming) {
      return "Waiting for confirmation...";
    }

    if (isSwitchingChain) {
      return "Switching chain...";
    }

    if (chain?.id !== transaction?.toChain.id) {
      return `Switch to ${transaction?.toChain.name}`;
    }

    return "Claim";
  }, [
    isPending,
    isConfirming,
    isSwitchingChain,
    isLoadingClaimTxParams,
    chain?.id,
    transaction?.toChain.id,
    transaction?.toChain.name,
  ]);

  const handleClaim = () => {
    if (transaction?.toChain.id && chain?.id && chain.id !== transaction?.toChain.id) {
      switchChain({ chainId: transaction.toChain.id });
      return;
    }

    if (claim) {
      claim();
    }
  };

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
              <Link
                href={`${transaction?.fromChain.blockExplorers?.default.url}/tx/${transaction?.bridgingTx}` || ""}
                target="_blank"
                rel="noopenner noreferrer"
              >
                {formatHex(transaction?.bridgingTx)}
              </Link>
              <ArrowRightIcon />
            </div>
          </li>
          <li>
            <span>{transaction?.toChain.name} Tx hash</span>
            <div className={styles.hash}>
              {transaction?.claimingTx ? (
                <Link
                  href={`${transaction?.toChain.blockExplorers?.default.url}/tx/${transaction.claimingTx}` || ""}
                  target="_blank"
                  rel="noopenner noreferrer"
                >
                  {formatHex(transaction.claimingTx)}
                </Link>
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
          <Button
            disabled={isLoadingClaimTxParams || isPending || isConfirming || isSwitchingChain}
            onClick={handleClaim}
            fullWidth
          >
            {buttonText}
          </Button>
        )}
      </div>
    </Modal>
  );
}
