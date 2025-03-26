import { formatUnits } from "viem";
import styles from "./received-amount.module.scss";
import { useTokenPrices } from "@/hooks";
import { useConfigStore, useChainStore, useFormStore } from "@/stores";
import { formatBalance } from "@/utils";
import { ETH_SYMBOL } from "@/constants";

function formatReceivedAmount(amount: bigint, tokenSymbol: string, bridgingFees: bigint) {
  if (tokenSymbol === ETH_SYMBOL) {
    return amount - bridgingFees;
  }
  return amount;
}

export default function ReceivedAmount() {
  const fromChain = useChainStore.useFromChain();
  const currency = useConfigStore.useCurrency();
  const amount = useFormStore((state) => state.amount);
  const token = useFormStore((state) => state.token);
  const bridgingFees = useFormStore((state) => state.bridgingFees);

  const { data: tokenPrices } = useTokenPrices([token[fromChain.layer]], fromChain.id);

  return (
    <div className={styles.value}>
      <p className={styles.crypto}>
        {formatBalance(formatUnits(formatReceivedAmount(amount || 0n, token.symbol, bridgingFees), token.decimals), 6)}{" "}
        {token.symbol}
      </p>
      {tokenPrices?.[token[fromChain.layer].toLowerCase()] &&
        tokenPrices?.[token[fromChain.layer].toLowerCase()] > 0 && (
          <p className={styles.amount}>
            {(
              Number(formatUnits(formatReceivedAmount(amount || 0n, token.symbol, bridgingFees), token.decimals)) *
              tokenPrices?.[token[fromChain.layer].toLowerCase()]
            ).toLocaleString("en-US", {
              style: "currency",
              currency: currency.label,
              maximumFractionDigits: 8,
            })}
          </p>
        )}
    </div>
  );
}
