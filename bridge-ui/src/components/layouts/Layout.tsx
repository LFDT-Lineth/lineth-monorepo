"use client";

import { usePathname } from "next/navigation";
import { useDynamicContext } from "@/lib/dynamic";
import Header from "../header";
import { useInitialiseChain } from "@/hooks";
import { Theme } from "@/types";
import Image from "next/image";
import styles from "./layout.module.scss";
import InternalNav from "../internal-nav";

export function Layout({ children }: { children: React.ReactNode }) {
  const { sdkHasLoaded } = useDynamicContext();
  useInitialiseChain();

  const pathname = usePathname();

  if (!sdkHasLoaded) {
    return <CommonLayout pathname={pathname}>{children}</CommonLayout>;
  }

  return <CommonLayout pathname={pathname}>{children}</CommonLayout>;
}

function CommonLayout({ children, pathname }: { children: React.ReactNode; pathname: string }) {
  return (
    <div className="layout">
      <div className="container-v2">
        <Header theme={Theme.navy} />
        <main>
          {pathname !== "/faq" && (
            <div className={styles["content-wrapper"]}>
              <InternalNav hide={pathname === "/"} />
            </div>
          )}
          {children}
        </main>
      </div>
      <div>
        <Image
          className="left-illustration"
          src={`${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/illustration-left.svg`}
          role="presentation"
          alt="illustration left"
          width={300}
          height={445}
          priority
        />
        <Image
          className="right-illustration"
          src={`${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/illustration-right.svg`}
          role="presentation"
          alt="illustration right"
          width={610}
          height={842}
          priority
        />
      </div>
    </div>
  );
}
