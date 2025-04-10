"use client";

import useFirstVisitModal from "@/hooks/useFirstVisitModal";
import styles from "./page.module.scss";
import { Widget } from "@/components/lifi/widget";

export default function Page() {
  const modal = useFirstVisitModal({ type: "all-bridges" });

  return (
    <section className={styles["content-wrapper"]}>
      <Widget />
      {modal}
    </section>
  );
}
