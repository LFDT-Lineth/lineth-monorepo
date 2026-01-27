import Link from "next/link";

import PlusIcon from "@/assets/icons/plus-square.svg";

import styles from "./list-your-app.module.scss";

export default function ListYourApp() {
  return (
    <Link href="https://developer.linea.build" target="_blank" className={styles["submit-dapp"]}>
      <PlusIcon />
      <span>List your app</span>
    </Link>
  );
}
