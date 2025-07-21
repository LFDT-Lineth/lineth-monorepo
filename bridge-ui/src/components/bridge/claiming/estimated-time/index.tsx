import { useState } from "react";
import dynamic from "next/dynamic";
import ClockIcon from "@/assets/icons/clock.svg";
import styles from "./estimated-time.module.scss";
import { useChainStore, useFormStore } from "@/stores";
import { getEstimatedTimeText } from "@/utils";

const EstimatedTimeModal = dynamic(() => import("../../modal/estimated-time"), {
  ssr: false,
});

export default function EstimatedTime() {
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const [showEstimatedTimeModal, setShowEstimatedTimeModal] = useState<boolean>(false);
  const estimatedTimeText = `~${getEstimatedTimeText(fromChain, token, { withSpaceAroundHyphen: false, isAbbreviatedTimeUnit: true })}`;

  return (
    <>
      <button type="button" className={styles.time} onClick={() => setShowEstimatedTimeModal(true)}>
        <ClockIcon />
        <span>{estimatedTimeText}</span>
      </button>
      {showEstimatedTimeModal && (
        <EstimatedTimeModal
          isModalOpen={showEstimatedTimeModal}
          onCloseModal={() => setShowEstimatedTimeModal(false)}
        />
      )}
    </>
  );
}
