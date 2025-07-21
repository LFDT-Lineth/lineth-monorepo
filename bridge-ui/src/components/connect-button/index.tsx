"use client";

import clsx from "clsx";
import { useDynamicContext } from "@dynamic-labs/sdk-react-core";
import styles from "./connect-button.module.scss";
import Button from "@/components/ui/button";

type ConnectButtonProps = {
  text: string;
  fullWidth?: boolean;
};

export default function ConnectButton({ text, fullWidth }: ConnectButtonProps) {
  const { setShowAuthFlow } = useDynamicContext();

  return (
    <Button
      className={clsx(styles["connect-btn"], {
        [styles["full-width"]]: fullWidth,
      })}
      onClick={() => setShowAuthFlow(true)}
    >
      {text}
    </Button>
  );
}
