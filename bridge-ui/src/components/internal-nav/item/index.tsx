import clsx from "clsx";
import Link from "next/link";

import ArrowRightIcon from "@/assets/icons/arrow-right.svg";
import CaretDownIcon from "@/assets/icons/caret-down.svg";

import styles from "./item.module.scss";

export type NavItemProps = {
  title: string;
  href: string;
  icon: React.ReactNode;
  label?: string;
  description: string;
  labelId?: string;
};

type Props = NavItemProps & {
  as?: "div" | "li";
  dropdown?: boolean;
  showCaret?: boolean;
  isOpen?: boolean;
};

export default function NavItem({
  title,
  description,
  href,
  icon,
  label,
  labelId,
  as,
  dropdown,
  showCaret,
  isOpen,
}: Props) {
  const Wrapper = as || "li";

  const content = (
    <>
      <div className={styles["card-wrapper"]}>
        <span className={styles["card-icon"]}>{icon}</span>
        <div className={styles["card-content"]}>
          <div className={styles["card-title-wrapper"]}>
            <h2 className={styles["card-title"]}>{title}</h2>
            {label && (
              <span id={labelId} className={styles["card-label"]}>
                {label}
              </span>
            )}
          </div>
          <p className={styles["card-description"]}>{description}</p>
        </div>
      </div>
      <span className={styles["right-arrow"]}>
        <ArrowRightIcon />
      </span>
      {showCaret && (
        <span className={styles["caret"]} aria-hidden="true">
          <CaretDownIcon />
        </span>
      )}
    </>
  );

  return (
    <Wrapper className={clsx(styles["card-item"], dropdown && styles["dropdown"], isOpen && styles["open"])}>
      {showCaret ? (
        <div data-testid={`nav-item-${title.split(" ").join("-")}`} className={styles["card-link"]}>
          {content}
        </div>
      ) : (
        <Link href={href} className={styles["card-link"]} data-testid={`nav-item-${title.split(" ").join("-")}`}>
          {content}
        </Link>
      )}
    </Wrapper>
  );
}
