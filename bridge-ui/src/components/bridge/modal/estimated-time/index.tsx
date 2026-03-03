import { getAdapter } from "@/adapters";
import Modal from "@/components/modal";
import Button from "@/components/ui/button";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { ChainLayer } from "@/types";
import { formatEstimatedTime } from "@/utils/format";

import styles from "./estimated-time.module.scss";

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function EstimatedTimeModal({ isModalOpen, onCloseModal }: Props) {
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const token = useFormStore((state) => state.token);
  const selectedMode = useFormStore((state) => state.selectedMode);

  const adapter = getAdapter(token, fromChain, toChain);
  const time = adapter?.getEstimatedTime?.(fromChain.layer, selectedMode ?? undefined);
  const estimatedTimeText = time ? formatEstimatedTime(time, { spacedHyphen: true }) : "";
  const estimatedTimeType = fromChain.layer === ChainLayer.L1 ? "deposit" : "withdraw";

  return (
    <Modal title="Estimated time" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          Linea has an approximate {estimatedTimeText} delay on {estimatedTimeType} as a security measure.
        </p>
        <Button fullWidth onClick={onCloseModal}>
          OK
        </Button>
      </div>
    </Modal>
  );
}
