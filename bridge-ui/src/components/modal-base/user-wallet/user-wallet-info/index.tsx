import CopyToClipboard from "@/components/copy-to-clipboard";
import UserAvatar from "@/components/user-avatar";
import clsx from "clsx";
import { useAccount } from "wagmi";
import { useEnsInfo } from "@/hooks/user/useEnsInfo";
import styles from "./user-wallet-info.module.scss";
import { shortenAddress } from "@/utils/format";

export default function UserWalletInfo() {
  const { address } = useAccount();
  const { ensName, ensAvatar } = useEnsInfo();
  return (
    <div className={styles.userInfo}>
      <UserAvatar src={ensAvatar ?? ""} />
      {ensName && (
        <div className={styles.labelWrapper}>
          <span className={clsx(styles.label, styles.primary)}>{ensName}</span>
          <CopyToClipboard text={ensName} />
        </div>
      )}
      {address && (
        <div className={styles.labelWrapper}>
          <span className={clsx(styles.label, !ensName && styles.primary)}>{shortenAddress(address)}</span>
          <CopyToClipboard text={address} />
        </div>
      )}
    </div>
  );
}
