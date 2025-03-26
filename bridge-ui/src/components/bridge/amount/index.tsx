import { ChangeEvent, useEffect, useState } from "react";
import { useAccount } from "wagmi";
import { formatUnits, parseUnits } from "viem";
import { useTokenPrices } from "@/hooks";
import { useChainStore, useConfigStore, useFormStore } from "@/stores";
import styles from "./amount.module.scss";

const AMOUNT_REGEX = /^[0-9]*[.,]?[0-9]*$/;
const MAX_AMOUNT_CHAR = 20;

export function Amount() {
  const currency = useConfigStore((state) => state.currency);
  const fromChain = useChainStore.useFromChain();
  const { address } = useAccount();

  const amount = useFormStore((state) => state.amount);
  const token = useFormStore((state) => state.token);
  const setAmount = useFormStore((state) => state.setAmount);

  const tokenAddress = token[fromChain.layer];

  const { data: tokenPrices } = useTokenPrices([tokenAddress], fromChain.id);

  const [inputValue, setInputValue] = useState(amount ? formatUnits(amount, token.decimals) : "");

  useEffect(() => {
    setInputValue(amount ? formatUnits(amount, token.decimals) : "");
  }, [amount, token.decimals]);

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    const { key } = event;
    const allowedKeys = ["Backspace", "Tab", "ArrowLeft", "ArrowRight", "Delete"];
    const decimalSeparators = [".", ","];

    // If the key pressed is a decimal separator, allow it only if none is already present.
    if (decimalSeparators.includes(key)) {
      if (decimalSeparators.some((sep) => inputValue.includes(sep))) {
        event.preventDefault();
      }
      return;
    }
    // Otherwise, allow digits and allowed control keys.
    if (!(/[0-9]/.test(key) || allowedKeys.includes(key))) {
      event.preventDefault();
    }
  };

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    let newValue = e.target.value;

    newValue = newValue.replace(/[,;]/g, ".");

    if (newValue.length > MAX_AMOUNT_CHAR) {
      newValue = newValue.substring(0, MAX_AMOUNT_CHAR);
    }

    if (newValue.length > 1 && newValue[0] === "0" && newValue[1] !== ".") {
      newValue = newValue.replace(/^0+/, "");
      if (newValue === "") newValue = "0";
    }

    if (!AMOUNT_REGEX.test(newValue)) {
      return;
    }

    setInputValue(newValue);

    if (newValue.endsWith(".")) {
      return;
    }

    try {
      const parsed = parseUnits(newValue, token.decimals);
      setAmount(parsed);
    } catch (error) {
      console.error("Error parsing amount:", error);
    }
  };

  useEffect(() => {
    setAmount(0n);
  }, [address, setAmount]);

  const formattedAmount = amount ? formatUnits(amount, token.decimals) : "";
  const tokenPrice = tokenPrices?.[tokenAddress.toLowerCase()];
  const calculatedValue =
    tokenPrice && tokenPrice > 0
      ? (Number(formattedAmount) * tokenPrice).toLocaleString("en-US", {
          style: "currency",
          currency: currency.label,
          maximumFractionDigits: 4,
        })
      : "";

  return (
    <div className={styles["amount"]}>
      <p className={styles.title}>Send</p>
      <input
        id="amount-input"
        type="text"
        autoCorrect="off"
        autoComplete="off"
        spellCheck="false"
        inputMode="decimal"
        value={inputValue}
        onKeyDown={handleKeyDown}
        onChange={handleChange}
        pattern={AMOUNT_REGEX.source}
        placeholder="0"
      />
      {!fromChain?.testnet && <span className={styles["calculated-value"]}>{calculatedValue}</span>}
    </div>
  );
}
