import dynamic from "next/dynamic";
import Image from "next/image";
import styles from "./with-fees.module.scss";
import { useState } from "react";
import { useFees } from "@/hooks";
import { useConfigStore } from "@/stores";
import { useFormattedDigit } from "@/hooks/useFormattedDigit";

const GasFees = dynamic(() => import("../../../modal/gas-fees"), {
  ssr: false,
});

type Props = {
  iconPath: string;
};

export default function WithFees({ iconPath }: Props) {
  const [showGasFeesModal, setShowGasFeesModal] = useState<boolean>(false);
  const currency = useConfigStore.useCurrency();

  const { total, fees, isLoading } = useFees();

  const formattedFees = useFormattedDigit(total.fees, 18);

  if (isLoading) {
    return null;
  }

  return (
    <>
      {total && (
        <button
          type="button"
          className={styles["gas-fees"]}
          onClick={() => {
            setShowGasFeesModal(true);
          }}
        >
          <Image src={iconPath} width={12} height={12} alt="fee-chain-icon" />
          <p className={styles["estimate-crypto"]}>{formattedFees} ETH</p>
          {total.fiatValue && (
            <p className={styles["estimate-amount"]}>{`(${total.fiatValue.toLocaleString("en-US", {
              style: "currency",
              currency: currency.label,
              maximumFractionDigits: 2,
            })})`}</p>
          )}
        </button>
      )}
      {showGasFeesModal && (
        <GasFees isModalOpen={showGasFeesModal} onCloseModal={() => setShowGasFeesModal(false)} fees={fees} />
      )}
    </>
  );
}
