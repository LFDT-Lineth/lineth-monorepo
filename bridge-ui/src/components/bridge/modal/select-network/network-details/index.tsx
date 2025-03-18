import Image from "next/image";
import styles from "./network-details.module.scss";

type Props = {
  name: string;
  image: string;
  onClickNetwork: () => void;
};

export default function NetworkDetails({ name, image, onClickNetwork }: Props) {
  return (
    <button className={styles.wrapper} type="button" onClick={onClickNetwork}>
      <Image className={styles.image} src={image} alt={name} width={24} height={24} />
      <p className={styles.name}>{name}</p>
    </button>
  );
}
