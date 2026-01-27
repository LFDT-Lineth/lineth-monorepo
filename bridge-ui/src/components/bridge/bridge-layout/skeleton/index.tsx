import clsx from "clsx";

import styles from "./skeleton.module.scss";

export default function BridgeSkeleton() {
  return (
    <div className={styles.wrapper}>
      <div className={styles.headline}>
        <div className={clsx(styles["action"], "pulsating")} />
      </div>
      <div className={styles.exchange}>
        <div className={clsx(styles["from"], "pulsating")} />
        <div className={clsx(styles["to"], "pulsating")} />
      </div>
      <div className={clsx(styles["amount-wrapper"], "pulsating")} />
      <div className={styles["connect-btn-wrapper"]}>
        <div className={clsx(styles["connect-btn"], "pulsating")} />
      </div>
    </div>
  );
}
