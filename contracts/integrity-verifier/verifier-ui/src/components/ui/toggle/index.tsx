"use client";

import { Switch } from "@headlessui/react";
import clsx from "clsx";
import styles from "./toggle.module.scss";

interface ToggleProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  description?: string;
  disabled?: boolean;
}

export function Toggle({ checked, onChange, label, description, disabled }: ToggleProps) {
  return (
    <Switch.Group as="div" className={styles.wrapper}>
      <div className={styles.labelContainer}>
        <Switch.Label className={styles.label}>{label}</Switch.Label>
        {description && <p className={styles.description}>{description}</p>}
      </div>
      <Switch
        checked={checked}
        onChange={onChange}
        disabled={disabled}
        className={clsx(styles.switch, checked && styles.checked)}
      >
        <span className={styles.thumb} />
      </Switch>
    </Switch.Group>
  );
}
