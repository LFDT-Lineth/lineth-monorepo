import React from "react";
import styles from "./bridge-mode.module.scss";
import Image from "next/image";
import { useFormStore } from "@/stores";
import CctpModeDropdown from "@/components/bridge/cctp-mode-dropdown";
import { isCctp } from "@/utils";

export default function BridgeMode() {
  const token = useFormStore((state) => state.token);

  if (isCctp(token)) {
    return <CctpModeDropdown />;
  }

  return (
    <div className={styles.container}>
      <div className={styles.button}>
        <div className={styles["selected-label"]}>
          <Image
            src={`${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/linea-rounded.svg`}
            width={16}
            height={16}
            alt="Native Bridge"
          />
          <span>Native Bridge</span>
        </div>
      </div>
    </div>
  );
}
