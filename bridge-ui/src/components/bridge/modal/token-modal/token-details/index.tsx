"use client";

import React, { memo, useCallback, useMemo } from "react";

import Image from "next/image";
import { formatUnits } from "viem";

import { useTokenBalance } from "@/hooks";
import { useChainStore } from "@/stores/chainStore";
import { type CurrencyOption } from "@/stores/configStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { useTokenStore } from "@/stores/tokenStoreProvider";
import { CCTPMode, Token } from "@/types";
import { formatBalance } from "@/utils/format";
import { isEth } from "@/utils/tokens";

import styles from "./token-details.module.scss";

interface TokenDetailsProps {
  isConnected: boolean;
  token: Token;
  onTokenClick: (token: Token) => void;
  tokenPrice?: number;
  currency: CurrencyOption;
}

const TokenDetails = memo(function TokenDetails({
  isConnected,
  token,
  onTokenClick,
  tokenPrice,
  currency,
}: TokenDetailsProps) {
  const setSelectedToken = useTokenStore((state) => state.setSelectedToken);
  const fromChain = useChainStore.useFromChain();
  const { balance } = useTokenBalance(token);
  const setToken = useFormStore((state) => state.setToken);
  const setAmount = useFormStore((state) => state.setAmount);
  const setCctpMode = useFormStore((state) => state.setCctpMode);

  const chainLayer = fromChain?.layer;
  const tokenNotFromCurrentLayer = chainLayer && !token[chainLayer] && !isEth(token);

  const formattedBalance = useMemo(() => formatUnits(balance, token.decimals), [balance, token.decimals]);

  const totalValue = useMemo(() => {
    if (tokenPrice !== undefined) {
      return tokenPrice * Number(formattedBalance);
    }
    return undefined;
  }, [formattedBalance, tokenPrice]);

  const handleClick = useCallback(() => {
    setAmount(0n);
    setSelectedToken(token);
    setToken(token);
    onTokenClick(token);
    setCctpMode(CCTPMode.STANDARD);
  }, [setAmount, setSelectedToken, setToken, token, onTokenClick, setCctpMode]);

  return (
    <button
      id={`token-details-${token.symbol}-btn`}
      data-testid={`token-details-${token.symbol.toLowerCase()}-btn`}
      className={styles["token-wrapper"]}
      type="button"
      disabled={tokenNotFromCurrentLayer}
      onClick={handleClick}
    >
      <div className={styles["left"]}>
        <Image src={token.image} alt={token.name} width={32} height={32} />
        <div className={styles["text-left"]}>
          <p className={styles["token-symbol"]}>{token.symbol}</p>
          <p className={styles["token-name"]}>{token.name}</p>
        </div>
      </div>
      {isConnected && !tokenNotFromCurrentLayer && (
        <div className={styles.rìght}>
          <p className={styles["balance"]} data-testid={`token-details-${token.symbol.toLowerCase()}-amount`}>
            {formatBalance(formattedBalance, 8)} {token.symbol}
          </p>
          {totalValue !== undefined && (
            <p className={styles["price"]}>
              {totalValue.toLocaleString("en-US", {
                style: "currency",
                currency: currency.label,
                maximumFractionDigits: 4,
              })}
            </p>
          )}
        </div>
      )}
      {tokenNotFromCurrentLayer && (
        <div className={styles["not-from-current-layer"]}>
          <p>Token is from other layer. Please swap networks to import token.</p>
        </div>
      )}
    </button>
  );
});

export default TokenDetails;
