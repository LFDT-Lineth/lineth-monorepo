import { useRouter } from "next/navigation";
import ArrowLeftIcon from "@/assets/icons/arrow-left.svg";
import clsx from "clsx";
import styles from "./page-back.module.scss";

type Props = {
  label?: string;
  href?: string;
  isHomepage?: boolean;
};

export default function PageBack({ label = "Back", href = "/", isHomepage = false }: Props) {
  const router = useRouter();

  const handleBack = () => {
    if (window.history.length <= 2) {
      router.push(href);
    } else {
      router.back();
    }
  };
  return (
    <div className={clsx(styles.wrapper, isHomepage && styles.homepage)}>
      <span className={styles.back} onClick={handleBack}>
        <ArrowLeftIcon />
        <span>{label}</span>
      </span>
    </div>
  );
}
