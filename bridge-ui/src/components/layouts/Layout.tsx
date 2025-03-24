"use client";

import clsx from "clsx";
import { usePathname } from "next/navigation";
import { useDynamicContext } from "@/lib/dynamic";
import Header from "../header";
import { useInitialiseChain } from "@/hooks";
import { Theme } from "@/types";
import Image from "next/image";

export function Layout({ children }: { children: React.ReactNode }) {
  const { sdkHasLoaded } = useDynamicContext();
  useInitialiseChain();

  const pathname = usePathname();

  if (!sdkHasLoaded) {
    return (
      <div className="layout">
        <div className="container-v2">
          <Header theme={Theme.navy} />
          <main>{children}</main>
        </div>
        <div>
          <Image
            className="left-illustration"
            src={"/images/illustration/illustration-left.svg"}
            role="presentation"
            alt="illustration left"
            width={300}
            height={445}
            priority
          />
          <Image
            className="right-illustration"
            src={"/images/illustration/illustration-right.svg"}
            role="presentation"
            alt="illustration right"
            width={610}
            height={842}
            priority
          />
          <Image
            className={clsx("mobile-illustration", { hidden: pathname === "/faq" })}
            src={"/images/illustration/illustration-mobile.svg"}
            role="presentation"
            alt="illustration mobile"
            width={0}
            height={0}
            style={{ width: "100%", height: "auto", objectFit: "cover" }}
            priority
          />
        </div>
      </div>
    );
  }

  return (
    <div className="layout">
      <div className="container-v2">
        <Header theme={Theme.navy} />
        <main>{children}</main>
      </div>

      <div>
        <Image
          className="left-illustration"
          src={"/images/illustration/illustration-left.svg"}
          role="presentation"
          alt="illustration left"
          width={300}
          height={445}
          priority
        />
        <Image
          className="right-illustration"
          src={"/images/illustration/illustration-right.svg"}
          role="presentation"
          alt="illustration right"
          width={610}
          height={842}
          priority
        />
        <Image
          className={clsx("mobile-illustration", { hidden: pathname === "/faq" })}
          src={"/images/illustration/illustration-mobile.svg"}
          role="presentation"
          alt="illustration mobile"
          width={0}
          height={0}
          style={{ width: "100%", height: "auto", objectFit: "cover" }}
          priority
        />
      </div>
    </div>
  );
}
