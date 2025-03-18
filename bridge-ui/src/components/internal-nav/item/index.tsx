import Link from "next/link";
import clsx from "clsx";
import styles from "./item.module.scss";

type Props = {
  title: string;
  href: string;
  active: boolean;
};

export default function NavItem({ title, href, active }: Props) {
  return (
    <Link
      href={href}
      className={clsx(styles["nav"], {
        [styles["active"]]: active,
      })}
    >
      <span>{title}</span>
    </Link>
  );
}
