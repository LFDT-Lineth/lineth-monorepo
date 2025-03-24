import { useState } from "react";
import Link from "next/link";
import UnionIcon from "@/assets/icons/union.svg";
import CloseIcon from "@/assets/icons/x-circle.svg";
import styles from "./top-banner.module.scss";
import Image from "next/image";

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
        src={"/images/illustration/banner/left.svg"}
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
        src={"/images/illustration/banner/right.svg"}
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
