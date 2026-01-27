import { useState } from "react";

import dynamic from "next/dynamic";

import AttentionIcon from "@/assets/icons/attention.svg";

import styles from "./manual-claim.module.scss";

const ManualClaimModal = dynamic(() => import("@/components/bridge/modal/manual-claim"), {
  ssr: false,
});

export default function ManualClaim() {
  const [showManualClaimModal, setShowManualClaimModal] = useState<boolean>(false);

  return (
    <>
      <button
        data-testid="manual-mode-btn"
        type="button"
        className={styles.manual}
        onClick={() => setShowManualClaimModal(true)}
      >
        <AttentionIcon />
        <span>Manual</span>
      </button>
      {showManualClaimModal && (
        <ManualClaimModal isModalOpen={showManualClaimModal} onCloseModal={() => setShowManualClaimModal(false)} />
      )}
    </>
  );
}
