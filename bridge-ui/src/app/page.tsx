"use client";

import styles from "./page.module.scss";
import { Widget } from "@/components/lifi/widget";

export default function Page() {
  return (
    <section className={styles["content-wrapper"]}>
      <Widget />
    </section>
  );
}
