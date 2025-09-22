"use client";

import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { useDevice } from "@/hooks";
import clsx from "clsx";
import BridgeAggregatorIcon from "@/assets/icons/bridge-aggregator.svg";
import NativeBridgeIcon from "@/assets/icons/native-bridge.svg";
import BuyIcon from "@/assets/icons/buy.svg";
import CentralizedExchangeIcon from "@/assets/icons/centralized-exchange.svg";
import Modal from "@/components/modal";
import NavItem, { NavItemProps } from "./item";
import styles from "./internal-nav.module.scss";

export const navList: NavItemProps[] = [
  {
    title: "Bridge Aggregator",
    description: "Bridge from any chain using the fastest route",
    icon: <BridgeAggregatorIcon />,
    label: "Fastest",
    href: "/bridge-aggregator",
  },
  {
    title: "Native Bridge",
    description: "Bridge from Ethereum via Linea’s official bridge",
    icon: <NativeBridgeIcon />,
    label: "No Fees",
    labelId: "no-fees-pill",
    href: "/native-bridge",
  },
  {
    title: "Centralized Exchange",
    description: "Transfer directly from any centralized exchange",
    icon: <CentralizedExchangeIcon />,
    href: "/centralized-exchange",
  },
  {
    title: "Buy",
    description: "Buy crypto with any payment methods",
    icon: <BuyIcon />,
    href: "/buy",
  },
];

export default function InternalNav({ hide }: { hide?: boolean }) {
  const pathname = usePathname();
  const { isMobile } = useDevice();
  const [isOpen, setIsOpen] = useState(false);
  const selected = useMemo(() => navList.find((item) => item.href === pathname), [pathname]);
  const currentList = useMemo(() => navList.filter((item) => item.href !== pathname), [pathname]);

  useEffect(() => {
    if (isMobile) return;
    const handleClickOutside = (event: MouseEvent) => {
      const dropdownElement = document.querySelector(`.${styles["dropdown-wrapper"]}`);
      if (dropdownElement && !dropdownElement.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isMobile]);

  useEffect(() => {
    setIsOpen(false);
  }, [pathname]);

  const toggleDropdown = () => setIsOpen((prev) => !prev);

  return (
    <div className={clsx(styles["wrapper"], { [styles["hide"]]: hide })}>
      {!hide && (
        <div className={styles["dropdown-wrapper"]}>
          {selected && (
            <button onClick={toggleDropdown}>
              <NavItem {...selected} as="div" dropdown showCaret isOpen={isOpen} />
            </button>
          )}
          {isMobile ? (
            <Modal title="" isOpen={isOpen} isDrawer onClose={() => setIsOpen(false)}>
              <div className={styles["modal-content"]}>
                <ul className={styles["list-nav-mobile"]}>
                  {navList.map((item) => (
                    <NavItem key={`internal-nav-item-${item.href}`} {...item} dropdown />
                  ))}
                </ul>
              </div>
            </Modal>
          ) : (
            <AnimatePresence>
              {isOpen && (
                <motion.ul
                  className={styles["list-nav"]}
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -10 }}
                  transition={{ duration: 0.2 }}
                >
                  {currentList.map((item) => (
                    <NavItem key={`internal-nav-item-${item.href}`} {...item} dropdown />
                  ))}
                </motion.ul>
              )}
            </AnimatePresence>
          )}
        </div>
      )}
    </div>
  );
}
