"use client";

import FaqHelp from "@/components/bridge/faq-help";
import styles from "./page.module.scss";
import { navList } from "@/components/internal-nav";
import NavItem from "@/components/internal-nav/item";

export default function Page() {
  return (
    <>
      <section className={styles["content-wrapper"]}>
        <h1 className={styles["title"]}>Fund Your Account</h1>
        <ul className={styles["cards-list"]}>
          {navList.map((item) => (
            <NavItem key={item.href} {...item} />
          ))}
        </ul>
      </section>
      <FaqHelp />
    </>
  );
}
