"use client";

import InternalNav from "@/components/internal-nav";
import styles from "./page.module.scss";
import { Widget } from "@/components/lifi/widget";

export default function Page() {
  return (
    <section className={styles["content-wrapper"]}>
      <InternalNav />
      <Widget />
    </section>
  );
}
