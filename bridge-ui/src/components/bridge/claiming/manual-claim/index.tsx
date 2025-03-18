import { useState } from "react";
import styles from "./manual-claim.module.scss";
import AttentionIcon from "@/assets/icons/attention.svg";
import ManualClaimModal from "@/components/bridge/modal/manual-claim";

export default function ManualClaim() {
  const [showManualClaimModal, setShowManualClaimModal] = useState<boolean>(false);

  return (
    <>
      <button type="button" className={styles.manual} onClick={() => setShowManualClaimModal(true)}>
        <AttentionIcon />
        <span>Manual</span>
      </button>
      <ManualClaimModal isModalOpen={showManualClaimModal} onCloseModal={() => setShowManualClaimModal(false)} />
    </>
  );
}
