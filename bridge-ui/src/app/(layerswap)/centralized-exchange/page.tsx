import PageFooter from "@/components/bridge/page-footer";
import { Widget } from "@/components/layerswap/widget";

import styles from "./page.module.scss";

export default function Page() {
  return (
    <>
      <section className={styles["content-wrapper"]}>
        <div className={styles["widget-wrapper"]}>
          <Widget />
        </div>
      </section>
      <PageFooter />
    </>
  );
}
