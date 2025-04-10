"use client";

import OnRamperWidget from "@/components/onramper";
import styles from "./page.module.scss";
import useFirstVisitModal from "@/hooks/useFirstVisitModal";

export default function Page() {
  const modal = useFirstVisitModal({ type: "buy" });

  return (
    <section className={styles["content-wrapper"]}>
      <OnRamperWidget />
      {modal}
    </section>
  );
}
