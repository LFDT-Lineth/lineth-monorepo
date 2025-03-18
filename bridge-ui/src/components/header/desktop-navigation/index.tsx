import { LinkBlock, Theme } from "@/types";
import Image from "next/image";
import Link from "next/link";

import styles from "./desktop-navigation.module.scss";
import clsx from "clsx";
import HeaderConnect from "@/components/header/header-connect";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";

type Props = {
  menus: LinkBlock[];
  theme?: Theme;
};

function filterMobileOnly(menu: LinkBlock) {
  return {
    ...menu,
    submenusLeft: (menu.submenusLeft || []).filter((item) => !item.mobileOnly),
  };
}

export const DesktopNavigation = ({ menus, theme = Theme.default }: Props) => {
  return (
    <nav className={styles["nav-wrapper"]}>
      <ul className={`${styles.navigation} ${styles[theme]}`}>
        {menus.map((menu, index) => (
          <MenuItem key={index} menu={filterMobileOnly(menu)} />
        ))}
        <li>
          <HeaderConnect />
        </li>
      </ul>
    </nav>
  );
};

type MenuItemProps = {
  menu: LinkBlock;
};

function MenuItem({ menu }: MenuItemProps) {
  const [showSubmenu, setShowsubmenu] = useState<boolean>(false);
  const pathname = usePathname();

  useEffect(() => {
    setShowsubmenu(false);
  }, [pathname]);

  return (
    <li
      className={clsx(styles.menuItem, {
        [styles["active"]]: menu.active,
        [styles["show"]]: showSubmenu && (menu.submenusLeft?.length || menu.submenusRight),
      })}
      onMouseEnter={() => {
        if (menu.submenusLeft?.length || menu.submenusRight) {
          setShowsubmenu(true);
        }
      }}
      onMouseLeave={() => {
        if (menu.submenusLeft?.length || menu.submenusRight) {
          setShowsubmenu(false);
        }
      }}
    >
      {menu.url && (
        <Link href={menu.url} target={menu.external ? "_blank" : "_self"}>
          <i className={styles.dot} />
          {menu.label}
        </Link>
      )}

      {!menu.url && (
        <>
          {menu.desktopUrl ? (
            <Link href={menu.desktopUrl} target={menu.external ? "_blank" : "_self"}>
              <i className={styles.dot} />
              {menu.label}
            </Link>
          ) : (
            <>
              <i className={styles.dot} />
              {menu.label}
            </>
          )}

          {menu.submenusLeft && (
            <ul className={styles.submenu}>
              {menu.submenusLeft.map((submenu, key) => (
                <li className={styles.submenuItem} key={key}>
                  <Link href={submenu.url as string} target={submenu.external ? "_blank" : "_self"}>
                    {submenu.label}
                    {submenu.external && (
                      <svg className={styles.newTab}>
                        <use href="#icon-new-tab" />
                      </svg>
                    )}
                  </Link>
                </li>
              ))}
              {menu.submenusRight && (
                <ul className={styles.right}>
                  {menu.submenusRight?.submenusLeft?.map((submenu, subIndex) => (
                    <li className={styles.submenuItem} key={subIndex}>
                      <Link
                        href={submenu.url as string}
                        target={submenu.external ? "_blank" : "_self"}
                        aria-label={submenu.label}
                        className={styles.iconItem}
                      >
                        <Image
                          src={submenu.icon?.file?.url as string}
                          width={submenu.icon?.file.details.image.width}
                          height={submenu.icon?.file.details.image.height}
                          alt={submenu.label}
                        />
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </ul>
          )}
        </>
      )}
    </li>
  );
}
