import Image from "next/image";
import { formatEther } from "viem";
import styles from "./with-fees.module.scss";
import { useState } from "react";
import GasFees from "../../../modal/gas-fees";
import { useFees } from "@/hooks";
import { useConfigStore } from "@/stores";

type Props = {
  iconPath: string;
};

export default function WithFees({ iconPath }: Props) {
  const [showGasFeesModal, setShowGasFeesModal] = useState<boolean>(false);
  const currency = useConfigStore.useCurrency();

  const { total, fees, isLoading } = useFees();

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
          <p className={styles["estimate-crypto"]}>{`${Number(formatEther(total.fees)).toFixed(8)} ETH`}</p>
          {total.fiatValue && (
            <p className={styles["estimate-amount"]}>{`(${total.fiatValue.toLocaleString("en-US", {
              style: "currency",
              currency: currency.label,
              maximumFractionDigits: 2,
            })})`}</p>
          )}
        </button>
      )}
      <GasFees isModalOpen={showGasFeesModal} onCloseModal={() => setShowGasFeesModal(false)} fees={fees} />
    </>
  );
}
