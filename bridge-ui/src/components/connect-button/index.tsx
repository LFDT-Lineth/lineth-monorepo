"use client";

import clsx from "clsx";
import styles from "./connect-button.module.scss";
import Button from "@/components/ui/button";
import { useConnect, useConnectors } from "wagmi";

type ConnectButtonProps = {
  text: string;
  fullWidth?: boolean;
};

export default function ConnectButton({ text, fullWidth }: ConnectButtonProps) {
  const connectors = useConnectors();
  const { connectAsync } = useConnect();

  const handleConnect = async () => {
    const web3authConnector = connectors.find((c) => c.id === "web3auth");
    if (web3authConnector) {
      await connectAsync({ connector: web3authConnector });
    }
  };

  return (
    <Button
      className={clsx(styles["connect-btn"], {
        [styles["full-width"]]: fullWidth,
      })}
      onClick={handleConnect}
    >
      {text}
    </Button>
  );
}
