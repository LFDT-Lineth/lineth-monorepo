import { formatEther } from "viem";
import styles from "./gas-fees-list-item.module.scss";
import { CurrencyOption } from "@/stores";

type Props = {
  name: string;
  fee: bigint;
  fiatValue: number | null;
  currency: CurrencyOption;
};

export default function GasFeesListItem({ name, fee, fiatValue, currency }: Props) {
  return (
    <li className={styles["list-item"]}>
      <span>{name}</span>
      <div className={styles["fee-row"]}>
        <span className={styles["fee-value"]}>{`${parseFloat(formatEther(fee)).toFixed(8)} ETH`}</span>
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
