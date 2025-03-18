import Image from "next/image";
import styles from "./bridge-two-logo.module.scss";

type Props = {
  src1: string;
  alt1: string;
  src2: string;
  alt2: string;
};

export default function BridgeTwoLogo({ src1, src2, alt1, alt2 }: Props) {
  return (
    <div className={styles["logo-wrapper"]}>
      <Image className={styles.big} src={src1} width="40" height="40" alt={alt1} />
      <Image className={styles.small} src={src2} width="16" height="16" alt={alt2} />
    </div>
  );
}
