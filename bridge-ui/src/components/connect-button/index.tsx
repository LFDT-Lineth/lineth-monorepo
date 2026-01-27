"use client";

import { useWeb3Auth, useWeb3AuthConnect } from "@web3auth/modal/react";
import clsx from "clsx";

import Button from "@/components/ui/button";

import styles from "./connect-button.module.scss";

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
