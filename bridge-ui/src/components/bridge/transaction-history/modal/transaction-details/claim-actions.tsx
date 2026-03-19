import { useEffect } from "react";

import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useConnection, useDisconnect, useSwitchChain } from "wagmi";

import Button from "@/components/ui/button";
import { useClaim } from "@/hooks";
import { BridgeTransaction } from "@/types";

import styles from "./transaction-details.module.scss";

type ClaimActionsProps = {
  transaction: BridgeTransaction;
  onCloseModal: () => void;
};

export default function ClaimActions({ transaction, onCloseModal }: ClaimActionsProps) {
  const { chain } = useConnection();
  const { mutate: disconnect } = useDisconnect();
  const {
    mutate: switchChain,
    isPending: isSwitchingChain,
    error: switchChainError,
    reset: resetSwitchChain,
  } = useSwitchChain();

  const {
    claim,
    claimContext,
    isClaimTxLoading,
    isSimulating,
    simulationFailed,
    isConfirming,
    isPending,
    isConfirmed,
    error: claimError,
  } = useClaim({ transaction });

  const queryClient = useQueryClient();
  useEffect(() => {
    if (isConfirmed) {
      queryClient.invalidateQueries({ queryKey: ["transactionHistory"], exact: false });
      onCloseModal();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isConfirmed]);

  const needsChainSwitch = chain?.id !== transaction.toChain.id;

  useEffect(() => {
    if (!needsChainSwitch) {
      resetSwitchChain();
    }
  }, [needsChainSwitch, resetSwitchChain]);

  const hasClaimOptions = !!claimContext?.claimOptions;

  const buttonText = (() => {
    if (isClaimTxLoading) return "Loading Claim Data...";
    if (isPending || isConfirming) return "Waiting for confirmation...";
    if (isSwitchingChain) return "Switching chain...";
    if (claimContext && !hasClaimOptions) return "Switch wallet";
    if (hasClaimOptions && !needsChainSwitch && isSimulating) return "Simulating claim...";
    if (hasClaimOptions && !needsChainSwitch) return claimContext!.label;
    if (needsChainSwitch) return `Switch to ${transaction.toChain.name}`;
    return "Claim";
  })();

  const isButtonDisabled =
    isClaimTxLoading ||
    isPending ||
    isConfirming ||
    isSwitchingChain ||
    (hasClaimOptions && !needsChainSwitch && (isSimulating || simulationFailed));

  const handleClaim = () => {
    if (transaction.toChain.id && chain?.id && chain.id !== transaction.toChain.id) {
      switchChain({ chainId: transaction.toChain.id });
      return;
    }
    if (claim) claim();
  };

  const handlePrimaryAction = () => {
    if (claimContext && !hasClaimOptions) {
      disconnect();
      return;
    }
    handleClaim();
  };

  return (
    <>
      <div className={styles.actions}>
        <Button disabled={isButtonDisabled} onClick={handlePrimaryAction} className={styles.action}>
          {buttonText}
        </Button>
        <Button variant="outline" onClick={onCloseModal} className={styles.action}>
          Cancel
        </Button>
      </div>
      {switchChainError && needsChainSwitch && (
        <p className={styles["error-text"]}>
          Chain switch failed. Please switch to {transaction.toChain.name} manually in your wallet and try again.
        </p>
      )}
      {claimError && <p className={styles["error-text"]}>Claim failed. Please try again.</p>}
      {simulationFailed && !needsChainSwitch && claimContext && (
        <p className={styles["error-text"]}>{claimContext.errorMessage}</p>
      )}
      {claimContext && claimContext.messages.length > 0 && (
        <div className={styles["claim-messages"]}>
          {claimContext.messages.map((msg, i) => (
            <p key={i} className={styles["helper-text"]}>
              {msg.text}
              {msg.link && (
                <>
                  {" "}
                  <Link href={msg.link.url} target="_blank" rel="noopener noreferrer">
                    {msg.link.label}
                  </Link>
                </>
              )}
            </p>
          ))}
        </div>
      )}
    </>
  );
}
