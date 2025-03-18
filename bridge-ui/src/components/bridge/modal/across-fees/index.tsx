import Modal from "@/components/modal";
import styles from "./across-fees.module.scss";
import Button from "@/components/ui/button";

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
};

export default function AcrossFees({ isModalOpen, onCloseModal }: Props) {
  const allFees = [
    {
      name: "Capital fee",
      fee1: "0.00007875 ETH",
      fee2: "$0.303",
    },
    {
      name: "LP fee",
      fee1: "0.00007875 ETH",
      fee2: "$0.303",
    },
    {
      name: "Relayer gas fee",
      fee1: "0.00007875 ETH",
      fee2: "$0.303",
    },
  ];
  return (
    <Modal title="Across fees" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          Fees paid to <span className={styles["across"]}>Across</span> and their liquidity providers and relayers
        </p>
        <ul className={styles.list}>
          {allFees.map((row, index) => (
            <li key={index}>
              <span>{row.name}</span>
              <div className={styles["two-fee"]}>
                <span className={styles["fee1"]}>{row.fee1}</span>
                <span className={styles["fee2"]}>{row.fee2}</span>
              </div>
            </li>
          ))}
        </ul>
        <Button fullWidth onClick={onCloseModal}>
          OK
        </Button>
      </div>
    </Modal>
  );
}
