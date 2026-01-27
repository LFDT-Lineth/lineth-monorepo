import CheckShieldIcon from "@/assets/icons/check-shield.svg";
import Modal from "@/components/modal";
import ToggleSwitch from "@/components/ui/toggle-switch";
import { useFormStore, useChainStore } from "@/stores";
import { ChainLayer, ClaimType } from "@/types";

import styles from "./advanced-settings.module.scss";

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function AdvancedSettings({ isModalOpen, onCloseModal }: Props) {
  const fromChain = useChainStore.useFromChain();

  const claim = useFormStore((state) => state.claim);
  const setClaim = useFormStore((state) => state.setClaim);

  return (
    <Modal title="Advanced settings" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <div className={styles["container"]}>
          <div className={styles.content}>
            <CheckShieldIcon className={styles.icon} />
            <div>
              <p className={styles["title"]}>Manual claim on destination</p>
              <p className={styles["text"]}>
                You will need to claim your transaction on the destination chain with an additional transaction that
                requires ETH on the destination chain.
              </p>
            </div>
          </div>
          <div className={styles.toggle}>
            <ToggleSwitch
              disabled={fromChain?.layer === ChainLayer.L2}
              checked={claim === ClaimType.MANUAL}
              onChange={(checked) => {
                if (checked) {
                  setClaim(ClaimType.MANUAL);
                } else {
                  setClaim(ClaimType.AUTO_PAID);
                }
              }}
            />
          </div>
        </div>
      </div>
    </Modal>
  );
}
