import React from "react";
import styles from "./bridge-mode.module.scss";
import Image from "next/image";
import { useFormStore } from "@/stores";
import { BridgeProvider } from "@/types";

export default function BridgeMode() {
  const token = useFormStore((state) => state.token);
  const label = token.bridgeProvider === BridgeProvider.NATIVE ? "Native bridge" : "CCTP";
  const logoSrc =
    token.bridgeProvider === BridgeProvider.NATIVE
      ? `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/linea-rounded.svg`
      : `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/cctp.svg`;

  return (
    <div className={styles.container}>
      <div className={styles.button}>
        <div className={styles["selected-label"]}>
          <Image src={logoSrc} width={16} height={16} alt={label} />
          <span>{label}</span>
        </div>
      </div>
    </div>
  );
}
