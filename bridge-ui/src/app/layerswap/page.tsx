import styles from "./page.module.scss";
import { Widget } from "@/components/layerswap/widget";

export default function Home() {
  return (
    <section className={styles["content-wrapper"]}>
      <Widget />
    </section>
  );
}
