"use client";

import { useEffect, useRef, useState } from "react";

import clsx from "clsx";
import { usePathname } from "next/navigation";
import { useAccount, useDisconnect } from "wagmi";

import Close from "@/assets/icons/close.svg";
import UserWalletInfo from "@/components/modal-base/user-wallet/user-wallet-info";
import PohCheck from "@/components/poh-check";
import SideBarMobile from "@/components/side-bar-mobile";
import Button from "@/components/ui/button";
import { useModal } from "@/contexts/ModalProvider";
import { useCheckPoh } from "@/hooks/useCheckPoh";

import styles from "./user-wallet.module.scss";

export enum PohStep {
  IDLE = "idle",
  SUMSUB_VERIFICATION = "sumsub-verification",
}

export function UserWallet() {
  const { disconnectAsync, isPending: isDisconnecting } = useDisconnect();
  const { updateModal, isModalOpen } = useModal();
  const { address } = useAccount();
  const { data: isHuman, refetch: refetchPoh, isLoading: isCheckingPoh } = useCheckPoh(address as string);
  const [step, setStep] = useState<PohStep>(PohStep.IDLE);
  const pathname = usePathname();
  const previousPathnameRef = useRef<string>(pathname);

  const handleDisconnect = async () => {
    try {
      await disconnectAsync();
      closeModal();
    } catch (error) {
      console.error("Disconnection failed:", error);
    }
  };

  const closeModal = () => {
    updateModal(false, "user-wallet");
  };

  useEffect(() => {
    if (!isModalOpen) {
      setStep(PohStep.IDLE);
    }
  }, [isModalOpen]);

  // Close modal only when pathname actually changes, not on initial mount
  useEffect(() => {
    if (previousPathnameRef.current !== pathname) {
      closeModal();
      previousPathnameRef.current = pathname;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname]);

  return (
    <div className={clsx(styles.container, step === PohStep.SUMSUB_VERIFICATION && styles.sumsubVerification)}>
      <div className={styles.header}>
        <Close className={styles.close} onClick={closeModal} role="button" />
      </div>

      <div className={styles.body}>
        <div className={styles.content}>
          {step === PohStep.IDLE && <UserWalletInfo />}
          {isModalOpen && (
            <PohCheck isHuman={!!isHuman} refetchPoh={refetchPoh} setStep={setStep} isCheckingPoh={isCheckingPoh} />
          )}
        </div>
        {step === PohStep.IDLE && (
          <Button
            variant="outline"
            className={styles.logoutButton}
            fullWidth
            disabled={isDisconnecting}
            onClick={handleDisconnect}
          >
            {isDisconnecting ? "Logging out..." : "Logout"}
          </Button>
        )}
      </div>
      <div className={styles.footer}>
        <SideBarMobile />
      </div>
    </div>
  );
}
