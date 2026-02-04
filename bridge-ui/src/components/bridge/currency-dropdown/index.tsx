import React, { memo, useState, useEffect, useRef, useCallback } from "react";

import CaretDownIcon from "@/assets/icons/caret-down.svg";
import { type CurrencyOption, useConfigStore } from "@/stores/configStore";

import styles from "./currency-dropdown.module.scss";

type Props = {
  disabled?: boolean;
};

const CurrencyDropdown = memo(function CurrencyDropdown({ disabled }: Props) {
  const supportedCurrencies = useConfigStore.useSupportedCurrencies();
  const currency = useConfigStore.useCurrency();
  const setCurrency = useConfigStore.useSetCurrency();

  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const handleSelect = useCallback(
    (option: CurrencyOption) => {
      setCurrency(option);
      setIsOpen(false);
    },
    [setCurrency],
  );

  const toggleDropdown = useCallback(() => {
    setIsOpen((prev) => !prev);
  }, []);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, []);

  return (
    <div className={styles.container}>
      <button ref={buttonRef} type="button" className={styles.button} onClick={toggleDropdown} disabled={disabled}>
        <div className={styles["selected-label"]}>
          <span className={styles.flag}>{currency.flag}</span>
          <span>{currency.label}</span>
        </div>
        <CaretDownIcon className={styles.caret} />
      </button>
      {isOpen && (
        <div ref={dropdownRef} className={styles.dropdown}>
          {supportedCurrencies.map((option) => (
            <div key={`currency-${option.value}`} onClick={() => handleSelect(option)} className={styles.option}>
              <span className={styles.flag}>{option.flag}</span>
              {option.label}
            </div>
          ))}
        </div>
      )}
    </div>
  );
});

export default CurrencyDropdown;
