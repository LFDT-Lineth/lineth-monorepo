import { type ReactNode } from "react";
import clsx from "clsx";
import styles from "./card.module.scss";

interface CardProps {
  children: ReactNode;
  className?: string;
  title?: string;
  description?: string;
}

export function Card({ children, className, title, description }: CardProps) {
  return (
    <section className={clsx(styles.card, className)}>
      {(title || description) && (
        <header className={styles.header}>
          {title && <h2 className={styles.title}>{title}</h2>}
          {description && <p className={styles.description}>{description}</p>}
        </header>
      )}
      <div className={styles.content}>{children}</div>
    </section>
  );
}
