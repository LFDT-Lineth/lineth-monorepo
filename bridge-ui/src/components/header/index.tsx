import { useRef } from "react";

import { useGSAP } from "@gsap/react";
import { gsap } from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import Link from "next/link";

import LineaLogo from "@/assets/logos/linea.svg";
import HeaderConnect from "@/components/header/header-connect";
import { useDevice } from "@/hooks";
import { LinkBlock } from "@/types";

import { DesktopNavigation } from "./desktop-navigation";
import styles from "./header.module.scss";
import { MobileNavigation } from "./mobile-navigation";

gsap.registerPlugin(ScrollTrigger);

type Props = {
  navData: LinkBlock[];
};

const Header = ({ navData }: Props) => {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const { isMobile } = useDevice();

  useGSAP(
    () => {
      if (!isMobile || !wrapperRef.current) return;

      const setter = gsap.quickSetter(wrapperRef.current, "--scroll-progress");
      let current = 0;
      let target = 0;
      let speed = 0.1;

      ScrollTrigger.create({
        trigger: document.body,
        start: "top top",
        end: "50px top",
        scrub: true,
        onUpdate: (self) => {
          target = self.progress;
          const velocity = Math.max(Math.abs(self.getVelocity()) / 1000, 0.3);
          speed = Math.min(velocity * 0.5, 1);
        },
      });

      const update = () => {
        const delta = target - current;
        current += delta * speed;

        if (Math.abs(delta) < 0.01) {
          current = target;
        }

        setter(current);
      };

      gsap.ticker.add(update);

      return () => {
        gsap.ticker.remove(update);
      };
    },
    { dependencies: [isMobile], scope: wrapperRef },
  );

  return (
    <div className={styles.wrapper} ref={wrapperRef}>
      <header className={styles.header}>
        <div>
          <Link href="https://linea.build/" className={styles["mobile-home"]} aria-label="Back to Home">
            <LineaLogo />
          </Link>
        </div>
        <div className={styles["header-right"]}>
          <div className={styles["header-actions"]}>
            <div className={styles["mobile-connect"]}>
              <HeaderConnect />
            </div>
            <div className="search-placeholder" />
          </div>
          <MobileNavigation menus={navData} />
        </div>
        <DesktopNavigation menus={navData} />
      </header>
    </div>
  );
};

export default Header;
