"use client";

import { RadioGroup } from "@headlessui/react";
import { useVerifierStore } from "@/stores/verifier";
import type { AdapterType } from "@/types";
import styles from "./header.module.scss";

const adapters: { value: AdapterType; label: string }[] = [
  { value: "viem", label: "Viem" },
  { value: "ethers", label: "Ethers" },
];

export function Header() {
  const { adapter, setAdapter, sessionId, reset } = useVerifierStore();

  return (
    <header className={styles.header}>
      <div className={styles.inner}>
        <div className={styles.titleSection}>
          <h1 className={styles.title}>Contract Integrity Verifier</h1>
          <p className={styles.subtitle}>Verify deployed smart contracts against local artifacts</p>
        </div>

        <div className={styles.controls}>
          <RadioGroup value={adapter} onChange={setAdapter} className={styles.adapterGroup}>
            <RadioGroup.Label className={styles.adapterLabel}>Web3 Library</RadioGroup.Label>
            <div className={styles.adapterOptions}>
              {adapters.map((option) => (
                <RadioGroup.Option
                  key={option.value}
                  value={option.value}
                  className={({ checked }) => `${styles.adapterOption} ${checked ? styles.checked : ""}`}
                >
                  {option.label}
                </RadioGroup.Option>
              ))}
            </div>
          </RadioGroup>

          <button onClick={reset} className={styles.resetButton} type="button" aria-label="Reset session">
            Reset
          </button>
        </div>
      </div>

      {sessionId && (
        <div className={styles.sessionInfo}>
          <span className={styles.sessionLabel}>Session:</span>
          <code className={styles.sessionId}>{sessionId.slice(0, 8)}...</code>
        </div>
      )}
    </header>
  );
}
