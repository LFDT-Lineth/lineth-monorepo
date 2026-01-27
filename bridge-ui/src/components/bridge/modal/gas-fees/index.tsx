import Modal from "@/components/modal";
import Button from "@/components/ui/button";

import GasFeesList from "./gas-fees-list";
import styles from "./gas-fees.module.scss";

type Props = {
  fees: {
    name: string;
    fee: bigint;
    fiatValue: number | null;
  }[];
  formattedCctpFees?: string;
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function GasFees({ isModalOpen, onCloseModal, fees, formattedCctpFees }: Props) {
  return (
    <Modal title="Gas fees" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          With blockchains you need to pay a fee to submit transactions. Transactions submitted to the network require a
          small amount of gas to ensure they&apos;re confirmed by the network.
        </p>
        <GasFeesList fees={fees} formattedCctpFees={formattedCctpFees} />
        <Button fullWidth onClick={onCloseModal}>
          OK
        </Button>
      </div>
    </Modal>
  );
}
