import { useMemo } from "react";

import { formatUnits } from "viem";

import { getAdapter } from "@/adapters";
import { useTokenPrices } from "@/hooks";
import useBridgeFees from "@/hooks/fees/useBridgeFees";
import { useChainStore } from "@/stores/chainStore";
import { useConfigStore } from "@/stores/configStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { formatBalance } from "@/utils/format";

import styles from "./received-amount.module.scss";

export default function ReceivedAmount() {
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const currency = useConfigStore.useCurrency();
  const amount = useFormStore((state) => state.amount);
  const token = useFormStore((state) => state.token);
  const { fees } = useBridgeFees();

  const adapter = getAdapter(token, fromChain, toChain);
  const { data: tokenPrices } = useTokenPrices([token[fromChain.layer]], fromChain.id);

  const receivedAmount = useMemo(() => {
    const raw =
      adapter?.computeReceivedAmount({
        amount: amount || 0n,
        token,
        fromChainLayer: fromChain.layer,
        fees,
      }) ??
      (amount || 0n);
    return formatUnits(raw, token.decimals);
  }, [adapter, amount, token, fromChain.layer, fees]);

  return (
    <div className={styles.value}>
      <p className={styles.crypto} data-testid="received-amount-text">
        {formatBalance(receivedAmount, 6)} {token.symbol}
      </p>
      {tokenPrices?.[token[fromChain.layer].toLowerCase()] &&
        tokenPrices?.[token[fromChain.layer].toLowerCase()] > 0 && (
          <p className={styles.amount}>
            {(Number(receivedAmount) * tokenPrices?.[token[fromChain.layer].toLowerCase()]).toLocaleString("en-US", {
              style: "currency",
              currency: currency.label,
              maximumFractionDigits: 8,
            })}
          </p>
        )}
    </div>
  );
}
