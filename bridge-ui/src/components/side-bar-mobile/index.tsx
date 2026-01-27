import clsx from "clsx";
import Link from "next/link";

import AppIcon from "@/assets/icons/app.svg";
import RewardsIcon from "@/assets/icons/reward.svg";
import TokensIcon from "@/assets/icons/tokens.svg";
import LineaIcon from "@/assets/logos/linea.svg";

import styles from "./side-bar-mobile.module.scss";

export default function SideBarMobile() {
  const navData = [
    {
      name: "Home",
      href: "https://linea.build/hub",
      icon: <LineaIcon />,
    },
    {
      name: "Tokens",
      href: "https://linea.build/hub/tokens",
      icon: <TokensIcon />,
    },
    {
      name: "Apps",
      href: "https://linea.build/hub/apps",
      icon: <AppIcon />,
    },
    {
      name: "Rewards",
      href: "https://linea.build/hub/rewards",
      icon: <RewardsIcon />,
    },
  ];
  return (
    <div className={styles.wrapper}>
      <nav className={styles["nav-container"]}>
        <ul>
          {navData.map((item, index) => (
            <li key={index} className={clsx(styles["nav-item"])}>
              <Link href={item.href} className={styles["nav-item-link"]}>
                <div className={styles.icon}>{item.icon}</div>
                {item.name}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}
