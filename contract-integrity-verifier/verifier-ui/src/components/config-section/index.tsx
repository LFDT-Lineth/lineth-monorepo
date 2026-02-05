"use client";

import { useVerifierStore } from "@/stores/verifier";
import { Card } from "@/components/ui/card";
import { Dropzone } from "@/components/ui/dropzone";
import { Button } from "@/components/ui/button";
import { ALLOWED_CONFIG_EXTENSIONS } from "@/lib/constants";
import styles from "./config-section.module.scss";

export function ConfigSection() {
  const { configFile, parsedConfig, configLoading, configError, uploadConfig, clearConfig } = useVerifierStore();

  const handleUpload = (file: File) => {
    uploadConfig(file);
  };

  if (parsedConfig) {
    return (
      <Card title="Configuration" description="Config file loaded successfully">
        <div className={styles.configInfo}>
          <div className={styles.fileName}>
            <svg
              className={styles.fileIcon}
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
            </svg>
            <div>
              <p className={styles.name}>{configFile?.name}</p>
              <p className={styles.meta}>{parsedConfig.format.toUpperCase()} format</p>
            </div>
          </div>

          <div className={styles.stats}>
            <div className={styles.stat}>
              <span className={styles.statValue}>{parsedConfig.chains.length}</span>
              <span className={styles.statLabel}>{parsedConfig.chains.length === 1 ? "Chain" : "Chains"}</span>
            </div>
            <div className={styles.stat}>
              <span className={styles.statValue}>{parsedConfig.contracts.length}</span>
              <span className={styles.statLabel}>{parsedConfig.contracts.length === 1 ? "Contract" : "Contracts"}</span>
            </div>
            <div className={styles.stat}>
              <span className={styles.statValue}>{parsedConfig.envVars.length}</span>
              <span className={styles.statLabel}>{parsedConfig.envVars.length === 1 ? "Variable" : "Variables"}</span>
            </div>
            <div className={styles.stat}>
              <span className={styles.statValue}>{parsedConfig.requiredFiles.length}</span>
              <span className={styles.statLabel}>{parsedConfig.requiredFiles.length === 1 ? "File" : "Files"}</span>
            </div>
          </div>

          <Button variant="ghost" size="sm" onClick={clearConfig}>
            Remove config
          </Button>
        </div>
      </Card>
    );
  }

  return (
    <Card title="Configuration" description="Upload your verifier config file (JSON or Markdown format)">
      <Dropzone
        accept={ALLOWED_CONFIG_EXTENSIONS}
        onUpload={handleUpload}
        label="Drop your config file here or click to browse"
        hint={`Accepts ${ALLOWED_CONFIG_EXTENSIONS.join(", ")} files`}
        loading={configLoading}
        error={configError || undefined}
      />
    </Card>
  );
}
