import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import Image from "next/image";

import { type AdapterMode } from "@/adapters";
import CaretDownIcon from "@/assets/icons/caret-down.svg";
import Modal from "@/components/modal";
import { useDevice } from "@/hooks";
import { useFormStore } from "@/stores/formStoreProvider";

import styles from "./bridge-mode-dropdown.module.scss";

type Props = {
  modes: readonly AdapterMode[];
  defaultMode: string;
};

export default function BridgeModeDropdown({ modes, defaultMode }: Props) {
  const { isMobile } = useDevice();
  const [isOpen, setIsOpen] = useState(false);
  const selectedMode = useFormStore((state) => state.selectedMode);
  const setSelectedMode = useFormStore((state) => state.setSelectedMode);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const activeId = selectedMode ?? defaultMode;
  const selectedOption = useMemo(() => modes.find((m) => m.id === activeId) ?? modes[0], [modes, activeId]);

  const handleSelect = useCallback(
    (mode: AdapterMode) => {
      setSelectedMode(mode.id);
      setIsOpen(false);
    },
    [setSelectedMode],
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
      modes.map((mode) => (
        <div key={`bridge-mode-${mode.id}`} onClick={() => handleSelect(mode)} className={styles.option}>
          <div className={styles.label}>{mode.label}</div>
          <div className={styles.description}>{mode.description}</div>
        </div>
      )),
    [modes, handleSelect],
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
