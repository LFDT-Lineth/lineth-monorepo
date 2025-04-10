"use client";

import BridgeLayout from "@/components/bridge/bridge-layout";
import styles from "./page.module.scss";
import useFirstVisitModal from "@/hooks/useFirstVisitModal";

export default function Home() {
  const modal = useFirstVisitModal({ type: "native-bridge" });

  return (
    <section className={styles["content-wrapper"]}>
      <BridgeLayout />
      {modal}
    </section>
  );
}
