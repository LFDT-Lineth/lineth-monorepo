import clsx from "clsx";
import Image from "@/components/ui/image";
import styles from "./user-avatar.module.scss";

type Props = {
  src: string;
  onClick?: () => void;
  size?: "small" | "default";
};

export default function UserAvatar({ src, onClick, size = "default" }: Props) {
  const content = <Image className={styles.image} src={src} alt="User avatar" width={60} height={60} unoptimized />;

  if (onClick) {
    return (
      <button
        className={clsx(styles.avatar, styles[size])}
        aria-label="Open user wallet info"
        type="button"
        onClick={onClick}
      >
        {content}
      </button>
    );
  }

  return <div className={clsx(styles.avatar, styles[size])}>{content}</div>;
}
