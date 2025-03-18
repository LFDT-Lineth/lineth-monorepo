import { useConfigStore } from "@/stores";
import GasFeesListItem from "./gas-fees-list-item";
import styles from "./gas-fees-list.module.scss";

type Props = {
  fees: {
    name: string;
    fee: bigint;
    fiatValue: number | null;
  }[];
};

export default function GasFeesList({ fees }: Props) {
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
    </ul>
  );
}
