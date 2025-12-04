"use client";

import clsx from "clsx";
import { useWeb3Auth, useWeb3AuthConnect } from "@web3auth/modal/react";
import styles from "./connect-button.module.scss";
import Button from "@/components/ui/button";

type ConnectButtonProps = {
  text: string;
  fullWidth?: boolean;
};

export default function ConnectButton({ text, fullWidth }: ConnectButtonProps) {
  const { connect, loading: isConnecting } = useWeb3AuthConnect();
  const { isInitializing } = useWeb3Auth();

  return (
    <Button
      disabled={isConnecting || isInitializing}
      className={clsx(styles["connect-btn"], {
        [styles["full-width"]]: fullWidth,
      })}
      onClick={connect}
    >
      {text}
    </Button>
  );
}
