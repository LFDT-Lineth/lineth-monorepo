import { JSX, useEffect, useState } from "react";

import clsx from "clsx";
import { motion, AnimatePresence } from "motion/react";
import { createPortal } from "react-dom";

import CloseIcon from "@/assets/icons/close.svg";

import styles from "./modal.module.scss";

type Props = {
  isOpen: boolean;
  onClose: () => void;
  children: JSX.Element;
  title: string;
  isDrawer?: boolean;
  size?: "md" | "lg";
  modalHeader?: JSX.Element;
};

const Modal = ({ isOpen, onClose, children, title, isDrawer = false, size = "md", modalHeader }: Props) => {
  const [mounted, setMounted] = useState<boolean>(false);

  useEffect(() => {
    setMounted(true);

    return () => setMounted(false);
  }, []);

  const overlayAnimation = {
    hidden: { opacity: 0 },
    visible: { opacity: 1 },
    exit: { opacity: 0 },
  };

  const drawerAnimation = {
    hidden: { y: "100%" },
    visible: { y: 0 },
    exit: { y: "100%" },
  };

  const modalAnimation = {
    hidden: { scale: 0.8 },
    visible: { scale: 1 },
    exit: { scale: 0.8 },
  };

  const modalContent = (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          className={styles.overlay}
          variants={overlayAnimation}
          initial="hidden"
          animate="visible"
          exit="exit"
          transition={{ duration: 0.2 }}
          onClick={onClose}
        >
          <motion.div
            className={clsx(isDrawer ? styles.drawer : styles.content, {
              [styles["md"]]: size === "md",
              [styles["lg"]]: size === "lg",
            })}
            variants={isDrawer ? drawerAnimation : modalAnimation}
            initial="hidden"
            animate="visible"
            exit="exit"
            transition={{ duration: 0.2 }}
            onClick={(e) => e.stopPropagation()}
          >
            {modalHeader || (
              <div className={styles.heading}>
                <div className={styles.title}>{title}</div>
                <div className={styles["close-icon"]} onClick={onClose} role="button">
                  <CloseIcon />
                </div>
              </div>
            )}
            {children}
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
  return mounted ? createPortal(modalContent, document.body) : null;
};

export default Modal;
