import styles from "./fees.module.scss";
import ManualClaim from "../manual-claim";
import EstimatedTime from "../estimated-time";
import WithFees from "./with-fees";
import { useFormStore, useChainStore } from "@/stores";

export default function Fees() {
  const fromChain = useChainStore.useFromChain();
  const claim = useFormStore((state) => state.claim);

  return (
    <>
      <div className={styles.estimate}>
        <WithFees iconPath={fromChain.iconPath} />
        <EstimatedTime />
        {claim === "manual" && <ManualClaim />}
      </div>
    </>
  );
}
