"use client";

import { useCallback, useEffect, useState } from "react";

import Image from "next/image";

import LineaIcon from "@/assets/logos/linea.svg";
import Modal from "@/components/modal";
import Button from "@/components/ui/button";
import { useConfigStore } from "@/stores/configStore";

import styles from "./tos-modal.module.scss";

const TOS_EFFECTIVE_DATE = "March 28, 2026";
const TOS_URL = "https://linea.build/upcoming-terms-of-service";
const ILLUSTRATION_SRC = `${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/bridge-first-time-modal-illustration.svg`;

const EXIT_ANIMATION_DURATION_MS = 300;

export default function TosModal() {
  const [showModal, setShowModal] = useState(false);
  const [shouldRenderModal, setShouldRenderModal] = useState(false);
  const rehydrated = useConfigStore.useRehydrated();
  const agreeToTerms = useConfigStore.useAgreeToTerms();
  const setAgreeToTerms = useConfigStore.useSetAgreeToTerms();

  useEffect(() => {
    if (rehydrated && !agreeToTerms) {
      setShowModal(true);
      setShouldRenderModal(true);
    }
  }, [rehydrated, agreeToTerms]);

  const handleAccept = useCallback(() => {
    setAgreeToTerms(true);
    setShowModal(false);
    setTimeout(() => {
      setShouldRenderModal(false);
    }, EXIT_ANIMATION_DURATION_MS);
  }, [setAgreeToTerms]);

  const noop = useCallback(() => {}, []);

  if (!shouldRenderModal) return null;

  return (
    <Modal title="" isOpen={showModal} onClose={noop} modalHeader={<ModalHeader />}>
      <div className={styles.body}>
        <p>
          We&rsquo;ve updated our{" "}
          <a href={TOS_URL} target="_blank" rel="noopener noreferrer" className={styles.link}>
            Terms of Service
          </a>{" "}
          &mdash; effective {TOS_EFFECTIVE_DATE}.
        </p>
        <p>By clicking Accept, you agree to be bound by the updated terms.</p>
        <Button data-testid="tos-modal-accept-btn" fullWidth onClick={handleAccept}>
          Accept
        </Button>
      </div>
    </Modal>
  );
}

function ModalHeader() {
  return (
    <div className={styles["header-wrapper"]}>
      <Image
        className={styles.illustration}
        src={ILLUSTRATION_SRC}
        width={128}
        height={179}
        alt="Bridge illustration"
      />
      <div className={styles["header-content"]}>
        <LineaIcon className={styles.logo} />
        <h3 className={styles.title}>Updated Terms of Service</h3>
      </div>
    </div>
  );
}
