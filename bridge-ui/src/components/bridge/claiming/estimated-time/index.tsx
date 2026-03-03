import { useState } from "react";

import dynamic from "next/dynamic";

import { getAdapter } from "@/adapters";
import ClockIcon from "@/assets/icons/clock.svg";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { formatEstimatedTime } from "@/utils/format";

import styles from "./estimated-time.module.scss";

const EstimatedTimeModal = dynamic(() => import("../../modal/estimated-time"), {
  ssr: false,
});

export default function EstimatedTime() {
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const token = useFormStore((state) => state.token);
  const selectedMode = useFormStore((state) => state.selectedMode);
  const [showEstimatedTimeModal, setShowEstimatedTimeModal] = useState<boolean>(false);

  const adapter = getAdapter(token, fromChain, toChain);
  const time = adapter?.getEstimatedTime?.(fromChain.layer, selectedMode ?? undefined);
  const estimatedTimeText = time ? `~${formatEstimatedTime(time, { abbreviated: true })}` : "";

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
