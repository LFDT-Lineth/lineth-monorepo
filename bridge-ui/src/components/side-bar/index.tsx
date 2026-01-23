import AppIcon from "@/assets/icons/app.svg";
import LineaFullLogo from "@/assets/logos/linea-full.svg";
import LineaIcon from "@/assets/logos/linea.svg";
import RewardsIcon from "@/assets/icons/reward.svg";
import TokensIcon from "@/assets/icons/tokens.svg";
import ListYourApp from "@/components/list-your-app";
import clsx from "clsx";
import Link from "next/link";
import styles from "./side-bar.module.scss";

const navItems = [
  {
    name: "Home",
    href: "https://linea.build/hub",
    icon: <LineaIcon />,
  },
  {
    name: "Tokens",
    href: "https://linea.build/hub/tokens",
    icon: <TokensIcon />,
    active: true,
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

export default function SideBar() {
  return (
    <div className={styles.wrapper}>
      <a href="https://linea.build/" className={styles.logo}>
        <LineaFullLogo />
      </a>
      <nav className={styles["nav-container"]}>
        <ul>
          {navItems.map((item, index) => (
            <li key={index} className={clsx(styles["nav-item"], item.active && styles["active"])}>
              <Link href={item.href} className={styles["nav-item-link"]}>
                <div className={styles.icon}>{item.icon}</div>
                {item.name}
              </Link>
            </li>
          ))}
        </ul>
        <ListYourApp />
      </nav>
    </div>
  );
}
