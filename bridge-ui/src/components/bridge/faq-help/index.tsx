import Link from "next/link";
import styles from "./faq-help.module.scss";
import clsx from "clsx";

type Props = {
  isMobile?: boolean;
};

export default function FaqHelp({ isMobile }: Props) {
  return (
    <div
      className={clsx(styles["faq-help"], {
        [styles["is-mobile"]]: isMobile,
        [styles["is-desktop"]]: !isMobile,
      })}
    >
      Need help? <Link href="/faq">Check our FAQ</Link>
    </div>
  );
}
