import FirstTimeVisitModal, { FirstTimeModalDataType } from "@/components/modal/first-time-visit";
import { useState, useEffect, useMemo } from "react";

type Props = {
  type: "all-bridges" | "native-bridge" | "buy";
};

const modalData: Record<string, FirstTimeModalDataType> = {
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
      src: "/images/illustration/bridge-first-time-modal-illustration.svg",
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
      src: "/images/illustration/bridge-first-time-modal-illustration.svg",
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
      src: "/images/illustration/buy-first-time-modal-illustration.svg",
      width: 157,
      height: 167,
    },
  },
};

const useFirstVisitModal = ({ type }: Props) => {
  const [showModal, setShowModal] = useState(false);
  const [shouldRenderModal, setShouldRenderModal] = useState(false);

  const data = useMemo(() => modalData[type], [type]);

  useEffect(() => {
    if (typeof window !== "undefined" && localStorage.getItem(`hasVisited-${type}`) !== "true") {
      setShowModal(true);
      setShouldRenderModal(true);
    }
  }, [type]);

  const closeModal = () => {
    setShowModal(false);
    setTimeout(() => {
      setShouldRenderModal(false);
    }, 300);
    if (typeof window !== "undefined") {
      localStorage.setItem(`hasVisited-${type}`, "true");
    }
  };

  return shouldRenderModal ? (
    <FirstTimeVisitModal isModalOpen={showModal} onCloseModal={closeModal} data={data} />
  ) : null;
};

export default useFirstVisitModal;
