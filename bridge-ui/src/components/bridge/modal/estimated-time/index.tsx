import Modal from "@/components/modal";
import styles from "./estimated-time.module.scss";
import Button from "@/components/ui/button";
import { useChainStore, useFormStore } from "@/stores";
import { getEstimatedTimeText } from "@/utils";
import { ChainLayer } from "@/types";

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function EstimatedTimeModal({ isModalOpen, onCloseModal }: Props) {
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const estimatedTimeText = getEstimatedTimeText(fromChain, token, {
    withSpaceAroundHyphen: true,
    isAbbreviatedTimeUnit: false,
  });
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
