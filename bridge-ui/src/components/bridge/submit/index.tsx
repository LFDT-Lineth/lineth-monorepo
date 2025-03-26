import { MouseEventHandler, useEffect, useMemo, useState } from "react";
import { useChainId, useSwitchChain } from "wagmi";
import Button from "@/components/ui/button";
import WalletIcon from "@/assets/icons/wallet.svg";
import styles from "./submit.module.scss";
import { useFormStore, useChainStore } from "@/stores";
import { useBridge } from "@/hooks";
import TransactionConfirmed from "../modal/transaction-confirmed";
import ConfirmDestinationAddress from "../modal/confirm-destination-address";

type Props = {
  setIsDestinationAddressOpen: MouseEventHandler<HTMLButtonElement>;
};

export function Submit({ setIsDestinationAddressOpen }: Props) {
  const [showTransactionConfirmedModal, setShowTransactionConfirmedModal] = useState<boolean>(false);
  const [showConfirmDestinationAddressModal, setShowConfirmDestinationAddressModal] = useState<boolean>(false);

  const fromChain = useChainStore.useFromChain();
  const amount = useFormStore((state) => state.amount);
  const balance = useFormStore((state) => state.balance);
  const recipient = useFormStore((state) => state.recipient);

  const resetForm = useFormStore((state) => state.resetForm);

  const { bridge, transactionType, isPending, isConfirming, isConfirmed, refetchAllowance } = useBridge();

  const chainId = useChainId();
  const { switchChain, isPending: isSwitchingChain } = useSwitchChain();

  const disabled = useMemo(() => {
    const originChainBalanceTooLow = amount && balance < amount;
    return originChainBalanceTooLow || !amount || amount <= 0n || isPending || isConfirming || isSwitchingChain;
  }, [amount, balance, isConfirming, isPending, isSwitchingChain]);

  const buttonText = useMemo(() => {
    if (!amount || amount <= 0n) {
      return "Enter an amount";
    }
    const originChainBalanceTooLow = amount && balance < amount;

    if (originChainBalanceTooLow) {
      return "Insufficient funds";
    }

    if (isPending || isConfirming) {
      return "Waiting for confirmation...";
    }

    if (isSwitchingChain) {
      return "Switching chain...";
    }

    if (transactionType === "approve") {
      return "Approve Token";
    }

    if (fromChain.id !== chainId) {
      return `Switch to ${fromChain.name}`;
    }

    return "Bridge";
  }, [
    amount,
    balance,
    chainId,
    fromChain.id,
    fromChain.name,
    isConfirming,
    isPending,
    isSwitchingChain,
    transactionType,
  ]);

  useEffect(() => {
    if (isConfirmed) {
      setShowTransactionConfirmedModal(true);
    }
  }, [isConfirmed]);

  return (
    <>
      <div className={styles.container}>
        <Button
          className={styles["submit-button"]}
          onClick={() => {
            if (fromChain.id !== chainId) {
              switchChain({ chainId: fromChain.id });
            } else {
              if (transactionType !== "approve") {
                setShowConfirmDestinationAddressModal(true);
              } else {
                bridge?.();
              }
            }
          }}
          disabled={disabled}
          fullWidth
        >
          {buttonText}
        </Button>
        <button type="button" className={styles["wallet-icon"]} onClick={setIsDestinationAddressOpen}>
          <WalletIcon />
        </button>
      </div>
      <ConfirmDestinationAddress
        isModalOpen={showConfirmDestinationAddressModal}
        recipient={recipient}
        onCloseModal={() => {
          setShowConfirmDestinationAddressModal(false);
        }}
        onConfirm={() => {
          bridge?.();
          setShowConfirmDestinationAddressModal(false);
        }}
      />
      <TransactionConfirmed
        isModalOpen={showTransactionConfirmedModal}
        transactionType={transactionType}
        onCloseModal={() => {
          if (transactionType !== "approve") {
            resetForm();
          } else {
            refetchAllowance?.();
          }
          setShowTransactionConfirmedModal(false);
        }}
      />
    </>
  );
}
