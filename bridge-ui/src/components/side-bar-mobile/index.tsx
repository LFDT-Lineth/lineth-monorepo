import DappIcon from "@/assets/icons/dapp.svg";
import TokenIcon from "@/assets/icons/token.svg";
import LineaIcon from "@/assets/logos/linea.svg";
import EventIcon from "@/assets/icons/event.svg";
import RewardsIcon from "@/assets/icons/reward.svg";
import clsx from "clsx";
import Link from "next/link";
import styles from "./side-bar-mobile.module.scss";

export default function SideBarMobile() {
  const navData = [
    {
      name: "Home",
      href: "https://linea.build/hub",
      icon: <LineaIcon />,
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
    },
    {
      name: "Events",
      href: "https://linea.build/hub/events",
      icon: <EventIcon />,
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
            <li
              key={index}
              className={clsx(
                styles["nav-item"],
                // item.href === pathname && styles["active"]
              )}
            >
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
