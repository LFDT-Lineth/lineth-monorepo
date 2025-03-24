import { ChangeEvent, useEffect, useState } from "react";
import { useAccount } from "wagmi";
import { isAddress } from "viem";
import clsx from "clsx";
import Link from "next/link";
import styles from "./destination-address.module.scss";
import XCircleIcon from "@/assets/icons/x-circle.svg";
import { useChainStore, useFormStore } from "@/stores";
import { ChainLayer } from "@/types";
import ArrowRightIcon from "@/assets/icons/arrow-right.svg";

export function DestinationAddress() {
  const { address } = useAccount();

  const toChain = useChainStore.useToChain();
  const recipient = useFormStore((state) => state.recipient);
  const setRecipient = useFormStore((state) => state.setRecipient);
  const [inputValue, setInputValue] = useState(recipient);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (inputValue && !isAddress(inputValue)) {
      setError("Invalid address");
    } else {
      setError(null);
    }
  }, [inputValue]);

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    setInputValue(() => e.target.value as `0x${string}`);
    if (isAddress(e.target.value)) {
      setRecipient(e.target.value);
    }
  };

  const handleResetInput = () => {
    if (address) {
      setInputValue(address);
      setRecipient(address);
    }
  };

  return (
    <div className={styles["destination-address"]}>
      <div className={styles["headline"]}>
        <p className={styles.title}>Send to wallet</p>
        {address !== inputValue && !error && isAddress(inputValue) && (
          <Link
            href={`${toChain.blockExplorers?.default.url ?? ""}/address/${inputValue}`}
            target="_blank"
            rel="noopenner noreferrer"
          >
            VIEW ON {toChain.layer === ChainLayer.L1 ? "ETHERSCAN" : "LINEASCAN"}
            <ArrowRightIcon />
          </Link>
        )}
      </div>

      <div className={styles["input-container"]}>
        <input
          type="text"
          id="address"
          required
          maxLength={42}
          value={inputValue}
          pattern="^0x[a-fA-F0-9]{40}$"
          onChange={handleChange}
          className={clsx(styles.input, {
            [styles["error"]]: error,
          })}
        />

        <button
          type="button"
          className={clsx(styles.reset, {
            [styles["show"]]: inputValue !== address,
          })}
          onClick={handleResetInput}
        >
          <XCircleIcon />
        </button>
      </div>

      <p
        className={clsx(styles["message-text"], {
          [styles["warning"]]: inputValue !== address,
          [styles["success"]]: inputValue === address,
          [styles["error"]]: error,
        })}
      >
        {error
          ? error.toString()
          : address !== inputValue
            ? "Editing the destination address can result in loss of your funds. Make sure you control this address."
            : "This is your connected address"}
      </p>
    </div>
  );
}
