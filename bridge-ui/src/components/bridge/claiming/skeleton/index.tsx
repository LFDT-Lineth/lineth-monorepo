import clsx from "clsx";

import styles from "./skeleton.module.scss";

export default function Skeleton() {
  return (
    <div className={styles.wrapper}>
      <div className={styles.result}>
        <div className={clsx(styles["logo-wrapper"])}>
          <div className={clsx(styles.big, "pulsating")} />
          <div className={clsx(styles.small, "pulsating")} />
        </div>
        <div className={clsx(styles.value, "pulsating")} />
      </div>
      <div className={clsx(styles.estimate, "pulsating")} />
    </div>
  );
}
