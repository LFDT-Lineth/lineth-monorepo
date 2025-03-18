import { formatUnits } from "viem";
import styles from "./received-amount.module.scss";
import { useTokenPrices } from "@/hooks";
import { useConfigStore, useChainStore, useFormStore } from "@/stores";

export default function ReceivedAmount() {
  const fromChain = useChainStore.useFromChain();
  const currency = useConfigStore.useCurrency();
  const amount = useFormStore((state) => state.amount);
  const token = useFormStore((state) => state.token);

  const { data: tokenPrices } = useTokenPrices([token[fromChain.layer]], fromChain.id);

  return (
    <div className={styles.value}>
      <p className={styles.crypto}>
        {formatUnits(amount || 0n, token.decimals)} {token.symbol}
      </p>
      {tokenPrices?.[token[fromChain.layer].toLowerCase()] &&
        tokenPrices?.[token[fromChain.layer].toLowerCase()] > 0 && (
          <p className={styles.amount}>
            {(
              Number(formatUnits(amount || 0n, token.decimals)) * tokenPrices?.[token[fromChain.layer].toLowerCase()]
            ).toLocaleString("en-US", {
              style: "currency",
              currency: currency.label,
              maximumFractionDigits: 4,
            })}
          </p>
        )}
    </div>
  );
}
