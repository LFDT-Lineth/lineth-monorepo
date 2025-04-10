import Modal from "@/components/modal";
import styles from "./first-time-visit.module.scss";
import Button from "@/components/ui/button";
import Image from "next/image";
import LineaIcon from "@/assets/logos/linea.svg";
import CloseIcon from "@/assets/icons/close.svg";

export type FirstTimeModalDataType = {
  title: string;
  description: string;
  steps: string[];
  btnText: string;
  extraText?: string;
  image: {
    src: string;
    width: number;
    height: number;
  };
};

type Props = {
  isModalOpen: boolean;
  onCloseModal: () => void;
  data: FirstTimeModalDataType;
};

export default function FirstTimeVisit({ isModalOpen, onCloseModal, data }: Props) {
  const { title, description, steps, btnText, extraText, image } = data;
  const modalHeader = (
    <div className={styles["header-wrapper"]}>
      <Image
        className={styles.illustration}
        src={image.src}
        width={image.width}
        height={image.height}
        role="presentation"
        alt="modal image"
      />
      <div className={styles["close-icon"]} onClick={onCloseModal} role="button">
        <CloseIcon />
      </div>
      <div className={styles["header-content"]}>
        <LineaIcon className={styles.logo} />
        <h3 className={styles.title}>{title}</h3>
      </div>
    </div>
  );

  return (
    <Modal title={title} isOpen={isModalOpen} onClose={onCloseModal} modalHeader={modalHeader}>
      <div className={styles["modal-inner"]}>
        <p className={styles.description}>{description}</p>
        <p className={styles.how}>How it works:</p>
        <ol className={styles.list}>
          {steps.map((step, index) => (
            <li key={index}>
              <span className={styles.order}>{index + 1}</span>
              <span>{step}</span>
            </li>
          ))}
        </ol>
        {extraText && <p className={styles.extra}>{extraText}</p>}
        <Button fullWidth onClick={onCloseModal}>
          {btnText}
        </Button>
      </div>
    </Modal>
  );
}
