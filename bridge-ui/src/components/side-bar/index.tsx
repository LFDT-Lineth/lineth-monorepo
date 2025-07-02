import DappIcon from "@/assets/icons/dapp.svg";
import TokenIcon from "@/assets/icons/token.svg";
import LineaFullLogo from "@/assets/logos/linea-full.svg";
import LineaIcon from "@/assets/logos/linea.svg";
import ListYourApp from "@/components/list-your-app";
import clsx from "clsx";
import Link from "next/link";
import { Fragment, useMemo } from "react";
import styles from "./side-bar.module.scss";

const navActiveColorList = [
  {
    bgColor: "var(--color-cyan)",
    textColor: "var(--color-black)",
  },
  {
    bgColor: "var(--color-indigo)",
    textColor: "var(--color-white)",
  },

  {
    bgColor: "var(--color-pink)",
    textColor: "var(--color-black)",
  },
  {
    bgColor: "var(--color-tangerine)",
    textColor: "var(--color-black)",
  },
  {
    bgColor: "var(--color-navy)",
    textColor: "var(--color-white)",
  },
];

export default function SideBar() {
  const filteredNavItems = useMemo(() => {
    const navItems = [
      {
        name: "Home",
        href: "https://linea.build/hub",
        icon: <LineaIcon />,
        external: false,
      },
      {
        name: "Apps",
        href: "https://linea.build/hub/apps",
        icon: <DappIcon />,
      },
      {
        name: "Tokens",
        href: "https://linea.build/hub/tokens",
        icon: <TokenIcon />,
        subItems: [
          {
            name: "Swap",
            href: "https://linea.build/hub/tokens/swap?toChain=59144&toToken=0xe5d7c2a44ffddf6b295a15c148167daaaf5cf34f",
          },
          {
            name: "Bridge",
            href: "/",
            active: true,
          },
        ],
      },
    ];

    return navItems;
  }, []);

  return (
    <div className={styles.wrapper}>
      <Link href="https://linea.build/" className={styles.logo}>
        <LineaFullLogo />
      </Link>
      <nav className={styles["nav-container"]}>
        <ul>
          {filteredNavItems.map((item, index) => (
            <Fragment key={index}>
              <li
                className={clsx(
                  styles["nav-item"],
                  // index !== 0 &&
                  //   pathname.startsWith(item.href) &&
                  //   !item.subItems?.some((subItem) => pathname === subItem.href) &&
                  //   styles["active"],
                  // index === 0 && pathname === item.href && styles["active"],
                )}
                style={
                  {
                    "--bg-color": navActiveColorList[index]?.bgColor,
                    "--text-color": navActiveColorList[index]?.textColor,
                  } as React.CSSProperties
                }
              >
                <Link
                  href={item.href}
                  className={styles["nav-item-link"]}
                  target={item.external ? "_blank" : undefined}
                >
                  <div className={styles.icon}>{item.icon}</div>
                  {item.name}
                </Link>
                {item.subItems && item.subItems.length > 0 && (
                  <ul className={styles["sub-items"]}>
                    {item.subItems.map((subItem, subIndex) => (
                      <li key={subIndex} className={clsx(styles["sub-item"], subItem.active && styles["active"])}>
                        <Link href={subItem.href}>{subItem.name}</Link>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            </Fragment>
          ))}
        </ul>
        <ListYourApp />
      </nav>
    </div>
  );
}
