"use client";
import { useEffect, useState } from "react";

import clsx from "clsx";
import Link from "next/link";
import { usePathname } from "next/navigation";

import UnionIcon from "@/assets/icons/union.svg";
import HeaderConnect from "@/components/header/header-connect";
import Image from "@/components/ui/image";
import { LinkBlock } from "@/types/index";

import styles from "./desktop-navigation.module.scss";

type Props = {
  menus: LinkBlock[];
};

export const DesktopNavigation = ({ menus }: Props) => {
  return (
    <nav className={styles["nav-wrapper"]}>
      <ul className={styles.navigation}>
        {menus.map((menu, index) => {
          const subMenuWithIcon = menu.submenusLeft?.filter((submenu) => submenu.icon);
          const subMenuWithoutIcon = menu.submenusLeft?.filter((submenu) => !submenu.icon);
          return (
            <MenuItem
              key={index}
              menu={menu}
              subMenuWithIcon={subMenuWithIcon}
              subMenuWithoutIcon={subMenuWithoutIcon}
            />
          );
        })}
        <li className={styles.connect}>
          <HeaderConnect />
        </li>
      </ul>
    </nav>
  );
};

type MenuItemProps = {
  menu: LinkBlock;
  subMenuWithIcon?: LinkBlock[];
  subMenuWithoutIcon?: LinkBlock[];
};

function MenuItem({ menu, subMenuWithIcon, subMenuWithoutIcon }: MenuItemProps) {
  const [showSubmenu, setShowsubmenu] = useState<boolean>(false);
  const pathname = usePathname();

  useEffect(() => {
    setShowsubmenu(false);
  }, [pathname]);

  return (
    <li
      className={clsx(styles.menuItem, menu.active && [styles["active"]], showSubmenu && [styles["show"]])}
      onMouseEnter={() => setShowsubmenu(true)}
      onMouseLeave={() => setShowsubmenu(false)}
    >
      {menu.url ? (
        <Link href={menu.url} target={menu.external ? "_blank" : "_self"}>
          <span className={styles.menuItemLabel}>
            <i className={styles.dot} />
            {menu.label}
          </span>
        </Link>
      ) : (
        <>
          <span className={styles.menuItemLabel}>
            <i className={styles.dot} />
            {menu.label}
          </span>

          {menu.submenusLeft && (
            <ul className={styles.submenu}>
              {subMenuWithoutIcon?.map((submenu, key) => (
                <li className={styles.submenuItem} key={key}>
                  <Link href={submenu.url as string} target={submenu.external ? "_blank" : "_self"}>
                    <div className={styles.submenuItemLabel}>
                      {submenu.label}
                      {submenu.external && <UnionIcon className={styles.newTab} />}
                    </div>
                    {submenu.text && (
                      <p
                        className={styles.subtext}
                        dangerouslySetInnerHTML={{
                          __html: submenu.text,
                        }}
                      />
                    )}
                  </Link>
                </li>
              ))}
              {subMenuWithIcon && subMenuWithIcon.length > 0 && (
                <li className={styles.submenuWithIcon}>
                  {subMenuWithIcon.map((submenu, index) => (
                    <Link key={index} href={submenu.url as string} target={submenu.external ? "_blank" : "_self"}>
                      <div className={styles.submenuItemLabel}>
                        <Image
                          className={styles.submenuIcon}
                          src={submenu.icon?.file.url as string}
                          width={submenu.icon?.file.details.image.width || 0}
                          height={submenu.icon?.file.details.image.height || 0}
                          alt={submenu.label}
                        />
                        <span>{submenu.label}</span>
                        {submenu.external && <UnionIcon className={styles.newTab} />}
                      </div>
                    </Link>
                  ))}
                </li>
              )}
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
                          src={submenu.icon?.file.url as string}
                          width={submenu.icon?.file.details.image.width || 0}
                          height={submenu.icon?.file.details.image.height || 0}
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
