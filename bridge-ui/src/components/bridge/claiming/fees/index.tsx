import { useFormStore, useChainStore } from "@/stores";
import { ClaimType } from "@/types";

import styles from "./fees.module.scss";
import EstimatedTime from "../estimated-time";
import ManualClaim from "../manual-claim";
import WithFees from "./with-fees";

export default function Fees() {
  const fromChain = useChainStore.useFromChain();
  const claim = useFormStore((state) => state.claim);

  return (
    <>
      <div className={styles.estimate}>
        <WithFees iconPath={fromChain.iconPath} />
        <EstimatedTime />
        {claim === ClaimType.MANUAL && <ManualClaim />}
      </div>
    </>
  );
}
