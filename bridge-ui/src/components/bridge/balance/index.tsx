import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useBlockNumber } from "wagmi";
import { formatBalance } from "@/utils";
import { useTokenBalance, useSelectedToken } from "@/hooks";
import styles from "./balance.module.scss";
import { useFormStore } from "@/stores";

export function Balance() {
  // Context
  const token = useSelectedToken();

  // Wagmi
  const queryClient = useQueryClient();
  const { data: blockNumber } = useBlockNumber({ watch: true });
  const { balance, queryKey } = useTokenBalance(token);

  // Form
  const setBalance = useFormStore((state) => state.setBalance);

  useEffect(() => {
    setBalance(balance);
  }, [balance, setBalance, token?.decimals]);

  useEffect(() => {
    if (blockNumber && blockNumber % 5n === 0n) {
      queryClient.invalidateQueries({ queryKey });
    }
  }, [blockNumber, queryClient, queryKey]);

  return (
    <div className={styles.balance}>
      <span>{balance && `${formatBalance(balance.toString())} ${token?.symbol}`} available</span>
    </div>
  );
}
