import clsx from "clsx";

import styles from "./layerswap-skeleton.module.scss";

export function LayerswapSkeleton() {
  return (
    <div className={styles.skeleton}>
      {/* Header with menu icon */}
      <div className={styles.header}>
        <div className={styles.menuIcon}>
          <span className="pulsating" />
          <span className="pulsating" />
          <span className="pulsating" />
        </div>
      </div>

      {/* Send from Section */}
      <div className={styles.section}>
        <div className={clsx(styles.sectionLabel, "pulsating")} />
        <div className={styles.card}>
          <div className={styles.cardContent}>
            <div className={clsx(styles.iconSmall, "pulsating")} />
            <div className={clsx(styles.textMedium, "pulsating")} />
          </div>
          <div className={clsx(styles.chevron, "pulsating")} />
        </div>
      </div>

      {/* Send to Section */}
      <div className={styles.section}>
        <div className={clsx(styles.sectionLabel, "pulsating")} />
        <div className={styles.card}>
          <div className={styles.cardContent}>
            <div className={styles.iconWithBadge}>
              <div className={clsx(styles.iconLarge, "pulsating")} />
              <div className={clsx(styles.iconBadge, "pulsating")} />
            </div>
            <div className={styles.textStack}>
              <div className={clsx(styles.textPrimary, "pulsating")} />
              <div className={clsx(styles.textSecondary, "pulsating")} />
            </div>
          </div>
          <div className={clsx(styles.chevron, "pulsating")} />
        </div>
      </div>

      {/* Wallet Address Row */}
      <div className={styles.card}>
        <div className={styles.cardContent}>
          <div className={clsx(styles.iconLarge, "pulsating")} />
          <div className={clsx(styles.textMedium, "pulsating")} />
        </div>
        <div className={clsx(styles.chevron, "pulsating")} />
      </div>

      {/* Enter amount Section */}
      <div className={styles.amountCard}>
        <div className={clsx(styles.amountLabel, "pulsating")} />
        <div className={styles.amountInput}>
          <div className={styles.amountValues}>
            <div className={clsx(styles.amountMain, "pulsating")} />
            <div className={clsx(styles.amountFiat, "pulsating")} />
          </div>
        </div>
      </div>

      {/* Button */}
      <div className={clsx(styles.button, "pulsating")} />

      {/* Footer */}
      <div className={styles.footer}>
        <div className={clsx(styles.footerContent, "pulsating")} />
      </div>
    </div>
  );
}
