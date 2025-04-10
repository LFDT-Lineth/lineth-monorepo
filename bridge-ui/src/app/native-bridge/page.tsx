"use client";

import BridgeLayout from "@/components/bridge/bridge-layout";
import styles from "./page.module.scss";

export default function Home() {
  return (
    <section className={styles["content-wrapper"]}>
      <BridgeLayout />
    </section>
  );
}
