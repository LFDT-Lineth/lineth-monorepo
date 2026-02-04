import { useState } from "react";

import dynamic from "next/dynamic";

import ClockIcon from "@/assets/icons/clock.svg";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { CCTPMode } from "@/types";
import { getEstimatedTimeText } from "@/utils/message";

import styles from "./estimated-time.module.scss";

const EstimatedTimeModal = dynamic(() => import("../../modal/estimated-time"), {
  ssr: false,
});

export default function EstimatedTime() {
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const cctpMode = useFormStore((state) => state.cctpMode);
  const [showEstimatedTimeModal, setShowEstimatedTimeModal] = useState<boolean>(false);
  const estimatedTimeText = `~${getEstimatedTimeText(fromChain, token, cctpMode ?? CCTPMode.STANDARD, { withSpaceAroundHyphen: false, isAbbreviatedTimeUnit: true })}`;

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
