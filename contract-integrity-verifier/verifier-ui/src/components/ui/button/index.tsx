import { type ButtonHTMLAttributes, type ReactNode } from "react";
import clsx from "clsx";
import styles from "./button.module.scss";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost";
  size?: "sm" | "md" | "lg";
  fullWidth?: boolean;
  loading?: boolean;
  children: ReactNode;
}

export function Button({
  variant = "primary",
  size = "md",
  fullWidth = false,
  loading = false,
  disabled,
  className,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      className={clsx(
        styles.button,
        styles[variant],
        styles[size],
        fullWidth && styles.fullWidth,
        loading && styles.loading,
        className,
      )}
      disabled={disabled || loading}
      {...props}
    >
      {loading && <span className={styles.spinner} aria-hidden="true" />}
      <span className={loading ? styles.hiddenText : undefined}>{children}</span>
    </button>
  );
}
