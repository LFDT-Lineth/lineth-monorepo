"use client";

import { usePathname } from "next/navigation";
import styles from "./internal-nav.module.scss";
import NavItem from "./item";

const NavData = [
  {
    title: "Native Bridge",
    href: "/",
  },
];

export default function InternalNav() {
  const pathnane = usePathname();

  return (
    <div className={styles["wrapper"]}>
      <div className={styles["list-nav"]}>
        {NavData.map((item, index) => (
          <NavItem key={index} {...item} active={pathnane === item.href} />
        ))}
      </div>
    </div>
  );
}
