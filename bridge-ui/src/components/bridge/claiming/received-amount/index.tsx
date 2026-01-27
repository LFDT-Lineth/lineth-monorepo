import { useMemo } from "react";

import { formatUnits } from "viem";

import { ETH_SYMBOL } from "@/constants";
import { useTokenPrices } from "@/hooks";
import { useCctpFee } from "@/hooks/transaction-args/cctp/useCctpUtilHooks";
import { useChainStore, useConfigStore, useFormStore } from "@/stores";
import { ChainLayer, Token } from "@/types";
import { formatBalance, isCctp } from "@/utils";

import styles from "./received-amount.module.scss";

function formatReceivedAmount(
  amount: bigint,
  token: Token,
  bridgingFees: bigint,
  minimumFees: bigint,
  fromChainLayer: ChainLayer,
  cctpFee: bigint | null,
) {
  if (isCctp(token)) {
    return cctpFee ? amount - cctpFee : amount;
  } else {
    if (token.symbol !== ETH_SYMBOL) {
      return amount;
    }

    const feesToApply = fromChainLayer === ChainLayer.L1 ? bridgingFees : minimumFees;

    return amount - feesToApply;
  }
}

export default function ReceivedAmount() {
  const fromChain = useChainStore.useFromChain();
  const currency = useConfigStore.useCurrency();
  const amount = useFormStore((state) => state.amount);
  const token = useFormStore((state) => state.token);
  const bridgingFees = useFormStore((state) => state.bridgingFees);
  const minimumFees = useFormStore((state) => state.minimumFees);
  const cctpFee = useCctpFee(amount, token.decimals);

  const { data: tokenPrices } = useTokenPrices([token[fromChain.layer]], fromChain.id);

  const receivedAmount = useMemo(
    () =>
      formatUnits(
        formatReceivedAmount(amount || 0n, token, bridgingFees, minimumFees, fromChain.layer, cctpFee),
        token.decimals,
      ),
    [amount, token, bridgingFees, minimumFees, fromChain.layer, cctpFee],
  );

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
