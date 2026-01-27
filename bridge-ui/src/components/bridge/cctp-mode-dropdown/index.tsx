import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import Image from "next/image";

import CaretDownIcon from "@/assets/icons/caret-down.svg";
import Modal from "@/components/modal";
import { useDevice } from "@/hooks";
import { useFormStore } from "@/stores";
import { CCTPMode } from "@/types";

import styles from "./cctp-mode-dropdown.module.scss";

type CCTPModeOption = {
  value: CCTPMode;
  label: string;
  description: string;
  logoSrc: string;
};

const cctpModeOptions: CCTPModeOption[] = [
  {
    value: CCTPMode.STANDARD,
    label: "CCTP Standard",
    description: "No fees. Slower settlement",
    logoSrc: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/cctp.svg`,
  },
  {
    value: CCTPMode.FAST,
    label: "CCTP Fast",
    description: "0.14% fee. Near-instant settlement",
    logoSrc: `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/cctp.svg`,
  },
];

export default function CctpModeDropdown() {
  const { isMobile } = useDevice();
  const [isOpen, setIsOpen] = useState(false);
  const cctpMode = useFormStore((state) => state.cctpMode);
  const setCctpMode = useFormStore((state) => state.setCctpMode);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const selectedOption = useMemo(
    () => cctpModeOptions.find((option) => option.value === cctpMode) || cctpModeOptions[0],
    [cctpMode],
  );

  const handleSelect = useCallback(
    (option: CCTPModeOption) => {
      setCctpMode(option.value);
      setIsOpen(false);
    },
    [setCctpMode, setIsOpen],
  );

  const toggleDropdown = () => {
    setIsOpen((prev) => !prev);
  };

  useEffect(() => {
    if (isMobile) return;
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
  }, [isMobile]);

  const options = useMemo(
    () =>
      cctpModeOptions.map((option) => (
        <div key={`cctp-mode-${option.value}`} onClick={() => handleSelect(option)} className={styles.option}>
          <div className={styles.label}>{option.label}</div>
          <div className={styles.description}>{option.description}</div>
        </div>
      )),
    [handleSelect],
  );

  return (
    <div className={styles.container}>
      <button ref={buttonRef} type="button" className={styles.button} onClick={toggleDropdown}>
        <div className={styles.selectedOption}>
          <Image
            className={styles.flag}
            src={selectedOption.logoSrc}
            width={16}
            height={16}
            alt={selectedOption.label}
          />
          <div className={styles.selectedLabel}>{selectedOption.label}</div>
        </div>
        <CaretDownIcon className={styles.caret} />
      </button>
      {isMobile ? (
        <Modal title="Transfer" isOpen={isOpen} isDrawer onClose={() => setIsOpen(false)}>
          <div className={styles.modalContent}>{options}</div>
        </Modal>
      ) : (
        isOpen && (
          <div ref={dropdownRef} className={styles.dropdown}>
            {options}
          </div>
        )
      )}
    </div>
  );
}
