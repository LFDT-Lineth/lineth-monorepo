"use client";

import styles from "./connect-button.module.scss";
import clsx from "clsx";
import Button from "@/components/ui/button";
import { useDynamicContext } from "@/lib/dynamic";

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
