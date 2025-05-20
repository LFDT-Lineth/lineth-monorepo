"use client";

import OnRamperWidget from "@/components/onramper";
import styles from "./page.module.scss";
import FaqHelp from "@/components/bridge/faq-help";

export default function Page() {
  return (
    <>
      <section className={styles["content-wrapper"]}>
        <OnRamperWidget />
      </section>
      <FaqHelp />
    </>
  );
}
