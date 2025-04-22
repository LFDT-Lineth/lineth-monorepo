import { formatEther } from "viem";
import styles from "./gas-fees-list-item.module.scss";
import { CurrencyOption } from "@/stores";
import { useMemo } from "react";

type Props = {
  name: string;
  fee: bigint;
  fiatValue: number | null;
  currency: CurrencyOption;
};

export default function GasFeesListItem({ name, fee, fiatValue, currency }: Props) {
  const feeText = useMemo(() => {
    if (fee === 0n) return "Free";
    return `${parseFloat(formatEther(fee)).toFixed(8)} ETH`;
  }, [fee]);

  return (
    <li className={styles["list-item"]}>
      <span>{name}</span>
      <div className={styles["fee-row"]}>
        <span className={styles["fee-value"]}>{feeText}</span>
        {fiatValue && (
          <span className={styles["fee-fiat-value"]}>
            {fiatValue.toLocaleString("en-US", {
              style: "currency",
              currency: currency.label,
              maximumFractionDigits: 2,
            })}
          </span>
        )}
      </div>
    </li>
  );
}
