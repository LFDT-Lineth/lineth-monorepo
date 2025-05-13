import Link from "next/link";
import clsx from "clsx";
import ArrowRightIcon from "@/assets/icons/arrow-right.svg";
import CaretDownIcon from "@/assets/icons/caret-down.svg";
import styles from "./item.module.scss";

export type NavItemProps = {
  title: string;
  href: string;
  icon: React.ReactNode;
  label?: string;
  description: string;
};

type Props = NavItemProps & {
  as?: "div" | "li";
  dropdown?: boolean;
  showCaret?: boolean;
  isOpen?: boolean;
};

export default function NavItem({ title, description, href, icon, label, as, dropdown, showCaret, isOpen }: Props) {
  const Wrapper = as || "li";
  return (
    <Wrapper key={href} className={clsx(styles["card-item"], dropdown && styles["dropdown"], isOpen && styles["open"])}>
      <Link href={href} className={styles["card-link"]}>
        <div className={styles["card-wrapper"]}>
          <span className={styles["card-icon"]}>{icon}</span>
          <div className={styles["card-content"]}>
            <div className={styles["card-title-wrapper"]}>
              <h2>{title}</h2>
              {label && <span className={styles["card-label"]}>{label}</span>}
            </div>
            <p className={styles["card-description"]}>{description}</p>
          </div>
        </div>
        <span className={styles["right-arrow"]}>
          <ArrowRightIcon />
        </span>
        {showCaret && (
          <span className={styles["caret"]}>
            <CaretDownIcon />
          </span>
        )}
      </Link>
    </Wrapper>
  );
}
