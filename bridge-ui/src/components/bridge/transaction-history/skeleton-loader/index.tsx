import clsx from "clsx";
import styles from "./skeleton-loader.module.scss";

export default function SkeletonLoader() {
  return (
    <div className={styles.container}>
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className={styles.group}>
          <div className={styles["skeleton-item"]}>
            {Array.from({ length: 2 }).map((_, i) => (
              <div key={i} className={styles["skeleton-group"]}>
                <div className={clsx(styles.skeleton, "pulsating")} />
                <div className={clsx(styles.skeleton, "pulsating")} />
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
