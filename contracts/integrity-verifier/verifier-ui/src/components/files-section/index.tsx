"use client";

import { useRef, useState } from "react";
import { useVerifierStore } from "@/stores/verifier";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ALLOWED_SCHEMA_EXTENSIONS, ALLOWED_ARTIFACT_EXTENSIONS } from "@/lib/constants";
import styles from "./files-section.module.scss";

export function FilesSection() {
  const { requiredFiles, uploadedFiles, fileUploadErrors, uploadFile, replaceFile } = useVerifierStore();

  const inputRefs = useRef<Map<string, HTMLInputElement>>(new Map());
  const replaceRefs = useRef<Map<string, HTMLInputElement>>(new Map());
  const [replacingFiles, setReplacingFiles] = useState<Set<string>>(new Set());

  if (requiredFiles.length === 0) {
    return null;
  }

  const handleFileSelect = async (originalPath: string, file: File | null) => {
    if (file) {
      await uploadFile(originalPath, file);
    }
  };

  const handleFileReplace = async (originalPath: string, file: File | null) => {
    if (file) {
      setReplacingFiles((prev) => new Set(prev).add(originalPath));
      try {
        await replaceFile(originalPath, file);
      } finally {
        setReplacingFiles((prev) => {
          const next = new Set(prev);
          next.delete(originalPath);
          return next;
        });
        // Reset the input so the same file can be selected again
        const input = replaceRefs.current.get(originalPath);
        if (input) input.value = "";
      }
    }
  };

  const getAcceptedExtensions = (type: "schema" | "artifact") => {
    return type === "schema" ? ALLOWED_SCHEMA_EXTENSIONS : ALLOWED_ARTIFACT_EXTENSIONS;
  };

  const uploadedCount = requiredFiles.filter((f) => f.uploaded).length;
  const totalCount = requiredFiles.length;

  return (
    <Card
      title="Required Files"
      description={`Upload schema and artifact files referenced in your config (${uploadedCount}/${totalCount} uploaded)`}
    >
      <div className={styles.fileList}>
        {requiredFiles.map((file) => {
          const uploaded = uploadedFiles.get(file.path);
          const error = fileUploadErrors.get(file.path);
          const accepted = getAcceptedExtensions(file.type);
          const isReplacing = replacingFiles.has(file.path);

          return (
            <div key={file.path} className={styles.fileItem}>
              <div className={styles.fileInfo}>
                <span className={styles.fileType}>{file.type}</span>
                <div className={styles.fileDetails}>
                  <code className={styles.filePath}>{file.path}</code>
                  <span className={styles.fileContract}>Required by: {file.contractName}</span>
                </div>
              </div>

              <div className={styles.fileActions}>
                {uploaded ? (
                  <div className={styles.uploadedRow}>
                    <span className={styles.uploaded}>
                      <svg
                        className={styles.checkIcon}
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        aria-hidden="true"
                      >
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                      <span className={styles.uploadedFilename}>{uploaded.filename}</span>
                    </span>
                    <input
                      type="file"
                      accept={accepted.join(",")}
                      onChange={(e) => handleFileReplace(file.path, e.target.files?.[0] || null)}
                      ref={(el) => {
                        if (el) replaceRefs.current.set(file.path, el);
                      }}
                      className={styles.hiddenInput}
                      id={`replace-${file.path}`}
                      disabled={isReplacing}
                    />
                    <div className={styles.replaceButtonWrapper}>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => replaceRefs.current.get(file.path)?.click()}
                        disabled={isReplacing}
                        className={styles.replaceButton}
                        aria-label="Upload to replace"
                      >
                        {isReplacing ? (
                          <span className={styles.spinner} aria-label="Replacing..." />
                        ) : (
                          <svg
                            className={styles.replaceIcon}
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            aria-hidden="true"
                          >
                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                            <polyline points="17 8 12 3 7 8" />
                            <line x1="12" y1="3" x2="12" y2="15" />
                          </svg>
                        )}
                      </Button>
                      <span className={styles.tooltip}>Upload to replace</span>
                    </div>
                  </div>
                ) : (
                  <>
                    <input
                      type="file"
                      accept={accepted.join(",")}
                      onChange={(e) => handleFileSelect(file.path, e.target.files?.[0] || null)}
                      ref={(el) => {
                        if (el) inputRefs.current.set(file.path, el);
                      }}
                      className={styles.hiddenInput}
                      id={`file-${file.path}`}
                    />
                    <Button variant="secondary" size="sm" onClick={() => inputRefs.current.get(file.path)?.click()}>
                      Upload
                    </Button>
                  </>
                )}
              </div>

              {error && (
                <p className={styles.error} role="alert">
                  {error}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </Card>
  );
}
