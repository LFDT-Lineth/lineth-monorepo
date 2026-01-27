import ArrowRightIcon from "@/assets/icons/arrow-right.svg";
import Tooltip from "@/components/ui/tooltip";
import { useFormStore, useChainStore } from "@/stores";

import styles from "./swap-chain.module.scss";

export default function SwapChain() {
  const switchChainInStore = useChainStore.useSwitchChain();
  const resetForm = useFormStore((state) => state.resetForm);

  return (
    <Tooltip text="Switch chains" position="top">
      <button
        className={styles["swap-chain"]}
        data-testid="swap-chain-button"
        onClick={(e) => {
          e.preventDefault();
          e.currentTarget.classList.toggle(styles["rotate-360"]);
          switchChainInStore();
          resetForm();
        }}
      >
        <ArrowRightIcon className={styles["arrow-icon"]} />
      </button>
    </Tooltip>
  );
}
