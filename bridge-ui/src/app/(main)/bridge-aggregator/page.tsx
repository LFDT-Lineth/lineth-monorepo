"use client";

import PageFooter from "@/components/bridge/page-footer";
import { Widget } from "@/components/lifi/widget";

import styles from "./page.module.scss";

export default function Page() {
  return (
    <>
      <section className={styles["content-wrapper"]}>
        <Widget />
      </section>
      <PageFooter />
    </>
  );
}
