import { ChangeEvent, useEffect, useState } from "react";

import clsx from "clsx";

import styles from "./toggle-switch.module.scss";

type Props = {
  checked?: boolean;
  onChange?: (checked: boolean) => void;
  disabled?: boolean;
};

export default function ToggleSwitch({ checked = false, onChange, disabled, ...rest }: Props) {
  const [isChecked, setIsChecked] = useState<boolean>(checked);

  useEffect(() => {
    setIsChecked(checked);
  }, [checked]);

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (!disabled) {
      setIsChecked(e.target.checked);

      if (onChange) {
        onChange(e.target.checked);
      }
    }
  };

  return (
    <label
      className={clsx(styles["switch"], {
        [styles["disabled"]]: disabled,
      })}
    >
      <input type="checkbox" checked={isChecked} onChange={handleChange} disabled={disabled} />
      <span className={styles["slider"]} {...rest} />
    </label>
  );
}
