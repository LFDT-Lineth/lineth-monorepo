import Link from "next/link";
import { LinkBlock } from "@/types";
import { useEffect, useState } from "react";
import Image from "next/image";
import HeaderConnect from "@/components/header/header-connect";

import styles from "./mobile-navigation.module.scss";
import LineaBridgeLogo from "@/assets/logos/linea-bridge.svg";
import clsx from "clsx";

type Props = {
  menus: LinkBlock[];
  theme?: "default" | "navy" | "cyan" | "indigo" | "tangerine";
  isAssociation?: boolean;
};

export const MobileNavigation = ({ menus, theme = "default" }: Props) => {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [activeMenu, setActiveMenu] = useState<number | null>(() => menus.findIndex((m) => m.active));

  useEffect(() => {
    if (isOpen) {
      document.body.classList.add("mobile-navigation-open");
      document.body.style.overflow = "hidden";
    } else {
      document.body.classList.remove("mobile-navigation-open");
      document.body.style.overflow = "";
    }

    return () => {
      document.body.classList.remove("mobile-navigation-open");
      document.body.style.overflow = "";
    };
  }, [isOpen]);

  const handleToggleMenu = (menu: LinkBlock, index: number) => {
    if (menu.url) {
      if (!menu.external) {
        setIsOpen(false);
      }
      return;
    }
    if (index !== -1) {
      setActiveMenu(index === activeMenu ? null : index);
    }
  };

  return (
    <div className={styles["nav-wrapper"]}>
      <HeaderConnect />
      <button onClick={() => setIsOpen(true)} className={`${styles.menuButton} ${styles[theme]}`}>
        <svg xmlns="http://www.w3.org/2000/svg" width="25" height="17" viewBox="0 0 25 17" fill="none">
          <line x1="0.469727" y1="1.48633" x2="24.0663" y2="1.48633" stroke="currentColor" />
          <line x1="0.469971" y1="8.75293" x2="24.0665" y2="8.75293" stroke="currentColor" />
          <line x1="0.469971" y1="16.0193" x2="24.0665" y2="16.0193" stroke="currentColor" />
        </svg>
      </button>
      {isOpen && (
        <div className={styles.wrapper}>
          <div className={styles.content}>
            <div className={styles.actions}>
              <Link href="/" aria-label="Go to homepage">
                <LineaBridgeLogo className={styles.logo} />
              </Link>
              <button onClick={() => setIsOpen(false)}>
                <svg
                  className={styles["close-icon"]}
                  width="24"
                  height="23"
                  viewBox="0 0 24 23"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <line x1="1.3193" y1="22.5245" x2="22.5695" y2="1.2743" stroke="currentColor" />
                  <line x1="2.02665" y1="1.27425" x2="23.2768" y2="22.5245" stroke="currentColor" />
                </svg>
              </button>
            </div>
            <div className={styles.navigationWrapper}>
              <ul className={styles.navigation}>
                {menus.map((menu, index) => (
                  <li
                    className={clsx(styles.menuItem, { [styles.active]: activeMenu === index })}
                    key={`mobile-menu-item-${index}`}
                    onClick={() => handleToggleMenu(menu, index)}
                  >
                    {menu.url ? (
                      <Link href={menu.url} target={menu.external ? "_blank" : "_self"}>
                        <span className={styles.label}>
                          <i className={styles.dot}></i>
                          {menu.label}
                          {menu.external && (
                            <svg className={styles.external}>
                              <use href="#icon-new-tab" />
                            </svg>
                          )}
                        </span>
                      </Link>
                    ) : (
                      <>
                        <span className={styles.label}>
                          <i className={styles.dot}></i>
                          {menu.label}
                        </span>
                        {menu.submenusLeft && (
                          <ul className={styles.submenu}>
                            {menu.submenusLeft.map((submenu, subIndex) => (
                              <li
                                className={styles.submenuItem}
                                key={`mobile-submenuleft-item-${subIndex}`}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleToggleMenu(submenu, -1);
                                }}
                              >
                                <Link href={submenu.url as string} target={submenu.external ? "_blank" : "_self"}>
                                  {submenu.label}
                                  {submenu.external && (
                                    <svg className={styles.external}>
                                      <use href="#icon-new-tab" />
                                    </svg>
                                  )}
                                </Link>
                              </li>
                            ))}
                          </ul>
                        )}
                        {menu.submenusRight && (
                          <ul className={`${styles.submenu} ${styles.right}`}>
                            {menu.submenusRight?.submenusLeft?.map((submenu, subIndex) => (
                              <li
                                className={styles.submenuItem}
                                key={`mobile-submenuright-submenuleft-item-${subIndex}`}
                              >
                                <Link
                                  href={submenu.url as string}
                                  target={submenu.external ? "_blank" : "_self"}
                                  aria-label={submenu.label}
                                  className={styles.iconItem}
                                >
                                  <Image
                                    src={submenu.icon?.file.url as string}
                                    width={submenu.icon?.file.details.image.width}
                                    height={submenu.icon?.file.details.image.height}
                                    alt={submenu.label}
                                  />
                                </Link>
                              </li>
                            ))}
                          </ul>
                        )}
                      </>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
