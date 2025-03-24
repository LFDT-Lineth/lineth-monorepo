"use client";

import InternalNav from "@/components/internal-nav";
import BridgeLayout from "@/components/bridge/bridge-layout";
import styles from "./page.module.scss";
import TopBanner from "@/components/top-banner";

export default function Home() {
  return (
    <>
      <TopBanner
        text="Bridging USDC (USDC.e) is temporarily disabled until March 26. Learn more in our announcement here."
        href="https://x.com/LineaBuild/status/1901347758230958528"
      />
      <section className={styles["content-wrapper"]}>
        <InternalNav />
        <BridgeLayout />
      </section>
    </>
  );
}
