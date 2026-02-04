"use client";
import { useCallback, useEffect, useState } from "react";

import Image from "next/image";
import { usePathname } from "next/navigation";

import CloseIcon from "@/assets/icons/close.svg";
import LineaIcon from "@/assets/logos/linea.svg";
import Modal from "@/components/modal";
import Button from "@/components/ui/button";
import { useConfigStore } from "@/stores/configStore";

import styles from "./first-time-visit.module.scss";

export type VisitedModalType = "all-bridges" | "native-bridge" | "buy";

const pathToModalType: Record<string, VisitedModalType> = {
  "/bridge-aggregator": "all-bridges",
  "/native-bridge": "native-bridge",
  "/buy": "buy",
};

type FirstTimeModalDataType = {
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

const modalData: Record<VisitedModalType, FirstTimeModalDataType> = {
  "all-bridges": {
    title: "Welcome to the Linea Bridge!",
    description: "Move your funds to Linea through the fastest route, at the lowest cost, and with no extra fees!",
    steps: [
      "Select your source chain & token",
      "Choose Linea as your destination",
      "Enter the amount & get the best rate",
      "Connect your wallet & bridge",
      "Your funds land on Linea in seconds",
    ],
    btnText: "Start bridging now",
    extraText: "Ready to bridge?",
    image: {
      src: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/bridge-first-time-modal-illustration.svg`,
      width: 128,
      height: 179,
    },
  },
  "native-bridge": {
    title: "Welcome to the Native Bridge!",
    description:
      "Ethereum to Linea using Linea’s official bridge. No third parties, no extra fees—just a direct way to move your assets.",
    steps: [
      "Select the token and amount you want to bridge from Ethereum to Linea.",
      "Connect your wallet & approve",
      "Confirm and wait - your funds land on Linea in about 20 minutes",
    ],
    btnText: "Start bridging now",
    extraText: "Ready to bridge?",
    image: {
      src: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/bridge-first-time-modal-illustration.svg`,
      width: 128,
      height: 179,
    },
  },
  buy: {
    title: "Fund Your Linea Wallet",
    description:
      "Buy tokens instantly at the best rates and with no extra fees. We compare multiple providers to find you the best rates and fastest transactions.",
    steps: [
      "Pick a token & amount",
      "Select a payment method (card, bank, etc.) and follow the instruction",
      "Connect your wallet",
      "Confirm & receive tokens in seconds",
    ],
    btnText: "Buy tokens now",
    image: {
      src: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/buy-first-time-modal-illustration.svg`,
      width: 157,
      height: 167,
    },
  },
};

export default function FirstVisitModal() {
  const pathname = usePathname();
  const [showModal, setShowModal] = useState(false);
  const [shouldRenderModal, setShouldRenderModal] = useState(false);
  const rehydrated = useConfigStore.useRehydrated();
  const visited = useConfigStore.useVisitedModal();
  const setVisitedModal = useConfigStore.useSetVisitedModal();

  const modalType = pathToModalType[pathname];
  const data = modalType ? modalData[modalType] : null;

  useEffect(() => {
    if (!modalType || !rehydrated) return;

    if (!visited[modalType]) {
      setShowModal(true);
      setShouldRenderModal(true);
    }
  }, [modalType, visited, rehydrated]);

  const handleClose = useCallback(() => {
    if (!modalType) return;

    setShowModal(false);
    setTimeout(() => {
      setShouldRenderModal(false);
    }, 300);
    setVisitedModal(modalType);
  }, [modalType, setVisitedModal]);

  if (!shouldRenderModal || !data) return null;

  return <BaseModal isModalOpen={showModal} onCloseModal={handleClose} data={data} />;
}

type BaseModalProps = {
  isModalOpen: boolean;
  onCloseModal: () => void;
  data: FirstTimeModalDataType;
};

function BaseModal({ isModalOpen, onCloseModal, data }: BaseModalProps) {
  const { title, description, steps, btnText, extraText, image } = data;

  return (
    <Modal
      title={title}
      isOpen={isModalOpen}
      onClose={onCloseModal}
      modalHeader={<ModalHeader image={image} title={title} onCloseModal={onCloseModal} />}
    >
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
        <Button data-testid="first-visit-modal-confirm-btn" fullWidth onClick={onCloseModal}>
          {btnText}
        </Button>
      </div>
    </Modal>
  );
}

type ModalHeaderProps = {
  onCloseModal: () => void;
  title: FirstTimeModalDataType["title"];
  image: FirstTimeModalDataType["image"];
};

function ModalHeader({ image, title, onCloseModal }: ModalHeaderProps) {
  return (
    <div className={styles["header-wrapper"]}>
      <Image
        className={styles.illustration}
        src={image.src}
        width={image.width}
        height={image.height}
        alt="modal image"
      />
      <div className={styles["close-icon"]} onClick={onCloseModal} role="button" aria-label="Close modal">
        <CloseIcon />
      </div>
      <div className={styles["header-content"]}>
        <LineaIcon className={styles.logo} />
        <h3 className={styles.title}>{title}</h3>
      </div>
    </div>
  );
}
