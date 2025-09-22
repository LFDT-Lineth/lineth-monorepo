import { useConfigStore } from "@/stores";
import GasFeesListItem from "./gas-fees-list-item";
import styles from "./gas-fees-list.module.scss";

type Props = {
  fees: {
    name: string;
    fee: bigint;
    fiatValue: number | null;
  }[];
  formattedCctpFees?: string;
};

export default function GasFeesList({ fees, formattedCctpFees }: Props) {
  const currency = useConfigStore.useCurrency();
  return (
    <ul className={styles.list}>
      {fees.map((row, index) => (
        <GasFeesListItem
          key={`gas-fees-list-item-${index}`}
          name={row.name}
          fee={row.fee}
          fiatValue={row.fiatValue}
          currency={currency}
        />
      ))}
      {formattedCctpFees && (
        <GasFeesListItem
          name="USDC fee"
          fee={0n}
          formattedCctpFees={formattedCctpFees}
          fiatValue={null}
          currency={currency}
        />
      )}
    </ul>
  );
}
