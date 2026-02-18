"use client";

import { useEffect } from "react";
import { useVerifierStore } from "@/stores/verifier";
import { Header } from "@/components/header";
import { ConfigSection } from "@/components/config-section";
import { FilesSection } from "@/components/files-section";
import { EnvVarsSection } from "@/components/env-vars-section";
import { OptionsSection } from "@/components/options-section";
import { ActionBar } from "@/components/action-bar";
import { ResultsSection } from "@/components/results-section";
import styles from "./page.module.scss";

export default function Home() {
  const { sessionId, initSession, restoreSession, parsedConfig, results } = useVerifierStore();

  // Initialize or restore session on mount
  useEffect(() => {
    if (sessionId) {
      restoreSession(sessionId);
    } else {
      initSession();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className={styles.container}>
      <Header />

      <div className={styles.content}>
        <ConfigSection />

        {parsedConfig && (
          <>
            <FilesSection />
            <EnvVarsSection />
            <OptionsSection />
            <ActionBar />
          </>
        )}

        {results && <ResultsSection />}
      </div>
    </div>
  );
}
