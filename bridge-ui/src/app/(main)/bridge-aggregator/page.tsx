"use client";

import FaqHelp from "@/components/bridge/faq-help";
import { Widget } from "@/components/lifi/widget";

import styles from "./page.module.scss";

export default function Page() {
  return (
    <>
      <section className={styles["content-wrapper"]}>
        <Widget />
      </section>
      <FaqHelp />
    </>
  );
}
