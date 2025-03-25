"use client";

import InternalNav from "@/components/internal-nav";
import BridgeLayout from "@/components/bridge/bridge-layout";
import styles from "./page.module.scss";

export default function Home() {
  return (
    <section className={styles["content-wrapper"]}>
      <InternalNav />
      <BridgeLayout />
    </section>
  );
}
