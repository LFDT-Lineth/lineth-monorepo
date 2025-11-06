"use client";

import { useEffect } from "react";
import clsx from "clsx";
import styles from "./modalBase.module.scss";
import { useModal } from "@/contexts/ModalProvider";
import { UserWallet } from "@/components/modal-base/user-wallet";

export function ModalBase() {
  const { isModalOpen, isModalType, modalData, updateModal } = useModal();
  const showMobileDrawer = ["bridge-nav"].includes(isModalType);

  const handleOnClick = (open: boolean) => {
    updateModal(open, isModalType, modalData);
  };

  useEffect(() => {
    if (isModalOpen) document.body.style.overflowY = "hidden";
    else document.body.style.overflowY = "auto";
  }, [isModalOpen]);

  return (
    <dialog className={styles.dialog} open={isModalOpen}>
      {/* panel */}
      <div className={clsx(styles.panel, isModalOpen && styles.open, showMobileDrawer && styles.drawer)}>
        {isModalType === "user-wallet" && <UserWallet />}
      </div>
      {/* overlay */}
      <div
        className={clsx(styles.overlay, {
          [styles.open as keyof typeof styles]: isModalOpen,
        })}
        onClick={() => handleOnClick(false)}
      />
    </dialog>
  );
}
