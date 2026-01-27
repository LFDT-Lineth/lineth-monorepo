import { useState } from "react";

import Image from "next/image";
import Link from "next/link";

import UnionIcon from "@/assets/icons/union.svg";
import CloseIcon from "@/assets/icons/x-circle.svg";

import styles from "./top-banner.module.scss";

type Props = {
  text: string;
  href: string;
};

export default function TopBanner({ text, href }: Props) {
  const [showBanner, setShowBanner] = useState<boolean>(true);

  const handleClose = () => {
    setShowBanner(false);
  };

  if (!showBanner) return null;

  return (
    <div className={styles["banner-wrapper"]}>
      <Image
        className={styles["left-illustration"]}
        src={`${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/banner/left.svg`}
        role="presentation"
        alt="banner illustration left"
        width={0}
        height={0}
        style={{ width: "56px", height: "100%" }}
        priority
      />
      <div className={styles["banner"]}>
        <Link href={href} target="_blank" rel="noopener noreferrer" className={styles["inner"]} passHref>
          <span>{text}</span>
          <UnionIcon className={styles["external-icon"]} />
        </Link>
      </div>
      <CloseIcon onClick={handleClose} className={styles["close-icon"]} />
      <Image
        className={styles["right-illustration"]}
        src={`${process.env.NEXT_PUBLIC_BASE_PATH}/images/illustration/banner/right.svg`}
        role="presentation"
        alt="banner illustration right"
        width={0}
        height={0}
        style={{ width: "221px", height: "100%" }}
        priority
      />
    </div>
  );
}
