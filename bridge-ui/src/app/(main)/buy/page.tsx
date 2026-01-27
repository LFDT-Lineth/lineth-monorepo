"use client";

import FaqHelp from "@/components/bridge/faq-help";
import OnRamperWidget from "@/components/onramper";

import styles from "./page.module.scss";

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
