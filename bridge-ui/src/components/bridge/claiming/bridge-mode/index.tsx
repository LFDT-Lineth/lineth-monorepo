import React from "react";

import Image from "next/image";

import { getAdapter } from "@/adapters";
import BridgeModeDropdown from "@/components/bridge/bridge-mode-dropdown";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";

import styles from "./bridge-mode.module.scss";

export default function BridgeMode() {
  const token = useFormStore((state) => state.token);
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const adapter = getAdapter(token, fromChain, toChain);

  if (adapter?.modes?.length) {
    return <BridgeModeDropdown modes={adapter.modes} defaultMode={adapter.defaultMode ?? adapter.modes[0].id} />;
  }

  return (
    <div className={styles.container}>
      <div className={styles.button}>
        <div className={styles["selected-label"]}>
          {adapter?.logoSrc && <Image src={adapter.logoSrc} width={16} height={16} alt={adapter.name} />}
          <span>{adapter?.name ?? "Native Bridge"}</span>
        </div>
      </div>
    </div>
  );
}
