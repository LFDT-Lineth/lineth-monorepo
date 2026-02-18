"use client";

import { useVerifierStore } from "@/stores/verifier";
import { Button } from "@/components/ui/button";
import styles from "./action-bar.module.scss";

export function ActionBar() {
  const { isReadyToVerify, runVerification, verifyStatus, verifyError } = useVerifierStore();

  const ready = isReadyToVerify();
  const isRunning = verifyStatus === "running";

  return (
    <div className={styles.actionBar}>
      <div className={styles.status}>
        {!ready && <p className={styles.hint}>Please upload all required files and fill in environment variables</p>}
        {verifyError && (
          <p className={styles.error} role="alert">
            {verifyError}
          </p>
        )}
      </div>

      <Button variant="primary" size="lg" onClick={runVerification} disabled={!ready || isRunning} loading={isRunning}>
        {isRunning ? "Running verification..." : "Run Verification"}
      </Button>
    </div>
  );
}
