import Modal from "@/components/modal";
import Button from "@/components/ui/button";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { ChainLayer, CCTPMode } from "@/types";
import { getEstimatedTimeText } from "@/utils/message";

import styles from "./estimated-time.module.scss";

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function EstimatedTimeModal({ isModalOpen, onCloseModal }: Props) {
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const cctpMode = useFormStore((state) => state.cctpMode);
  const estimatedTimeText = getEstimatedTimeText(fromChain, token, cctpMode ?? CCTPMode.STANDARD, {
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
