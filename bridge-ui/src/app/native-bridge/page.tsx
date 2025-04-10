"use client";

import BridgeLayout from "@/components/bridge/bridge-layout";
import styles from "./page.module.scss";
import FirstVisitModal from "@/components/modal/first-time-visit";

export default function Home() {
  return (
    <section className={styles["content-wrapper"]}>
      <BridgeLayout />
      <FirstVisitModal type="native-bridge" />
    </section>
  );
}
