import { MouseEventHandler, useEffect, useMemo, useState } from "react";

import clsx from "clsx";
import dynamic from "next/dynamic";
import { useAccount, useChainId, useSwitchChain } from "wagmi";

import WalletIcon from "@/assets/icons/wallet.svg";
import ConnectButton from "@/components/connect-button";
import Button from "@/components/ui/button";
import { useBridge } from "@/hooks";
import { useFormStore, useChainStore } from "@/stores";

import styles from "./submit.module.scss";

type Props = {
  isDestinationAddressOpen: boolean;
  setIsDestinationAddressOpen: MouseEventHandler<HTMLButtonElement>;
};

const ConfirmDestinationAddress = dynamic(() => import("../modal/confirm-destination-address"), {
  ssr: false,
});

const TransactionConfirmed = dynamic(() => import("../modal/transaction-confirmed"), {
  ssr: false,
});

export function Submit({ isDestinationAddressOpen, setIsDestinationAddressOpen }: Props) {
  const { address, isConnected } = useAccount();

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

  const needChainSwitch = useMemo(() => {
    return fromChain.id !== chainId;
  }, [fromChain.id, chainId]);

  const disabled = useMemo(() => {
    if (needChainSwitch) return false;
    const originChainBalanceTooLow = amount && balance < amount;
    return originChainBalanceTooLow || !amount || amount <= 0n || isPending || isConfirming || isSwitchingChain;
  }, [amount, balance, isConfirming, isPending, isSwitchingChain, needChainSwitch]);

  const buttonText = useMemo(() => {
    // Do not prompt user for action when in a loading state
    if (isPending || isConfirming) {
      return "Waiting for confirmation...";
    }

    if (isSwitchingChain) {
      return "Switching chain...";
    }

    // Do not let user do actions with wallet connected to wrong chain
    if (needChainSwitch) {
      return `Switch to ${fromChain.name}`;
    }

    if (!amount || amount <= 0n) {
      return "Enter an amount";
    }
    const originChainBalanceTooLow = amount && balance < amount;

    if (originChainBalanceTooLow) {
      return "Insufficient funds";
    }

    if (transactionType === "approve") {
      return "Approve Token";
    }

    return "Bridge";
  }, [amount, balance, fromChain.name, isConfirming, isPending, isSwitchingChain, transactionType, needChainSwitch]);

  useEffect(() => {
    if (isConfirmed) {
      setShowTransactionConfirmedModal(true);
    }
  }, [isConfirmed]);

  return (
    <>
      <div className={styles.container}>
        {isConnected ? (
          <Button
            className={styles["submit-button"]}
            onClick={() => {
              if (needChainSwitch) {
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
        ) : (
          <ConnectButton fullWidth text={"Connect wallet"} />
        )}
        <button
          type="button"
          className={clsx(styles["wallet-icon"], {
            [styles["active"]]: isDestinationAddressOpen || (recipient !== address && isConnected),
          })}
          onClick={setIsDestinationAddressOpen}
        >
          <WalletIcon />
        </button>
      </div>
      {showConfirmDestinationAddressModal && (
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
      )}
      {showTransactionConfirmedModal && (
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
      )}
    </>
  );
}
