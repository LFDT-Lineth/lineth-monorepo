import clsx from "clsx";
import { useRouter } from "next/navigation";

import ArrowLeftIcon from "@/assets/icons/arrow-left.svg";

import styles from "./page-back.module.scss";

type Props = {
  label?: string;
};

export default function PageBack({ label = "Back" }: Props) {
  const router = useRouter();

  const handleBack = () => {
    router.push("/");
  };

  return (
    <div className={clsx(styles.wrapper)}>
      <span className={styles.back} onClick={handleBack}>
        <ArrowLeftIcon />
        <span>{label}</span>
      </span>
    </div>
  );
}
