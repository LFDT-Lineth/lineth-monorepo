"use client";

import OnRamperWidget from "@/components/onramper";
import styles from "./page.module.scss";
import FirstVisitModal from "@/components/modal/first-time-visit";

export default function Page() {
  return (
    <section className={styles["content-wrapper"]}>
      <OnRamperWidget />
      <FirstVisitModal type="buy" />
    </section>
  );
}
