"use client";

import styles from "./page.module.scss";
import { Widget } from "@/components/lifi/widget";
import FirstVisitModal from "@/components/modal/first-time-visit";

export default function Page() {
  return (
    <section className={styles["content-wrapper"]}>
      <Widget />
      <FirstVisitModal type="all-bridges" />
    </section>
  );
}
