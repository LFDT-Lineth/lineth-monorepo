"use client";

import { useDynamicContext } from "@dynamic-labs/sdk-react-core";
import { usePathname } from "next/navigation";
import { useInitialiseChain } from "@/hooks";
import { LinkBlock } from "@/types";
import Header from "@/components/header";
import InternalNav from "@/components/internal-nav";
import SideBar from "@/components/side-bar";
import SideBarMobile from "@/components/side-bar-mobile";
import PageBack from "@/components/page-back";
import styles from "./layout.module.scss";
import { isHomePage } from "@/utils";

export function Layout({ children, navData }: { children: React.ReactNode; navData: LinkBlock[] }) {
  const { sdkHasLoaded } = useDynamicContext();
  useInitialiseChain();

  const pathname = usePathname();

  if (!sdkHasLoaded) {
    return (
      <CommonLayout navData={navData} pathname={pathname}>
        {children}
      </CommonLayout>
    );
  }

  return (
    <CommonLayout navData={navData} pathname={pathname}>
      {children}
    </CommonLayout>
  );
}

function CommonLayout({
  children,
  pathname,
  navData,
}: {
  children: React.ReactNode;
  pathname: string;
  navData: LinkBlock[];
}) {
  return (
    <div className={styles.layout}>
      <div className={styles.container}>
        <SideBar />
        <SideBarMobile />
        <div className={styles.right}>
          <Header navData={navData} />
          <main>
            {!isHomePage(pathname) && <PageBack />}
            {pathname !== "/faq" && (
              <div className={styles["content-wrapper"]}>
                <InternalNav hide={isHomePage(pathname)} />
              </div>
            )}
            {children}
          </main>
        </div>
      </div>
    </div>
  );
}
