import Link from "next/link";
import PlusIcon from "@/assets/icons/plus-square.svg";
import styles from "./list-your-app.module.scss";

export default function ListYourApp() {
  return (
    <Link
      href="https://2urwb.share.hsforms.com/2M7Q9cFIWQxyZgLdocN3Smg?submissionGuid=07ed5477-53c1-498a-a5e7-41b12999d66c"
      target="_blank"
      className={styles["submit-dapp"]}
    >
      <PlusIcon />
      <span>List your app</span>
    </Link>
  );
}
