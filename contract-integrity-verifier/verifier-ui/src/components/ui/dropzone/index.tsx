"use client";

import { useCallback, useState, type DragEvent, type ChangeEvent } from "react";
import clsx from "clsx";
import styles from "./dropzone.module.scss";

interface DropzoneProps {
  accept: string[];
  onUpload: (file: File) => void;
  label: string;
  hint?: string;
  disabled?: boolean;
  loading?: boolean;
  error?: string;
}

export function Dropzone({ accept, onUpload, label, hint, disabled, loading, error }: DropzoneProps) {
  const [isDragging, setIsDragging] = useState(false);

  const handleDragOver = useCallback(
    (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (!disabled && !loading) {
        setIsDragging(true);
      }
    },
    [disabled, loading],
  );

  const handleDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragging(false);

      if (disabled || loading) return;

      const files = e.dataTransfer.files;
      if (files.length > 0) {
        const file = files[0];
        const ext = `.${file.name.split(".").pop()?.toLowerCase()}`;

        if (accept.includes(ext)) {
          onUpload(file);
        }
      }
    },
    [accept, onUpload, disabled, loading],
  );

  const handleChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      const files = e.target.files;
      if (files && files.length > 0) {
        onUpload(files[0]);
      }
      // Reset input so same file can be selected again
      e.target.value = "";
    },
    [onUpload],
  );

  return (
    <div
      className={clsx(
        styles.dropzone,
        isDragging && styles.dragging,
        disabled && styles.disabled,
        loading && styles.loading,
        error && styles.hasError,
      )}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <input
        type="file"
        accept={accept.join(",")}
        onChange={handleChange}
        disabled={disabled || loading}
        className={styles.input}
        id="file-upload"
        aria-describedby={hint ? "file-hint" : undefined}
      />
      <label htmlFor="file-upload" className={styles.label}>
        {loading ? (
          <span className={styles.spinner} aria-hidden="true" />
        ) : (
          <svg
            className={styles.icon}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            aria-hidden="true"
          >
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <polyline points="17 8 12 3 7 8" />
            <line x1="12" y1="3" x2="12" y2="15" />
          </svg>
        )}
        <span className={styles.text}>{label}</span>
        {hint && (
          <span id="file-hint" className={styles.hint}>
            {hint}
          </span>
        )}
      </label>
      {error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
