import { useEffect, useState, useMemo } from "react";
import { useAccount } from "wagmi";
import BridgeTwoLogo from "@/components/bridge/bridge-two-logo";
import styles from "./claiming.module.scss";
import SettingIcon from "@/assets/icons/setting.svg";
import AdvancedSettings from "@/components/bridge/modal/advanced-settings";
import Skeleton from "@/components/bridge/claiming/skeleton";
import ReceivedAmount from "./received-amount";
import Fees from "./fees";
import { useFormStore, useChainStore } from "@/stores";
import BridgeMode from "./bridge-mode";
import { ChainLayer } from "@/types";
import { isCctp } from "@/utils";

export default function Claiming() {
  const { isConnected } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const [loading, setLoading] = useState<boolean>(false);
  const [showAdvancedSettingsModal, setShowAdvancedSettingsModal] = useState<boolean>(false);

  const amount = useFormStore((state) => state.amount);
  const balance = useFormStore((state) => state.balance);
  const token = useFormStore((state) => state.token);

  const originChainBalanceTooLow = amount && balance < amount;

  useEffect(() => {
    setLoading(true);
    const timeout = setTimeout(() => {
      setLoading(false);
    }, 1000);

    return () => clearTimeout(timeout);
  }, [amount]);

  // Do not allow user to go to AdvancedSettings modal, when they have no choice of ClaimType anyway
  const showSettingIcon = useMemo(() => {
    if (fromChain.layer === ChainLayer.L2) return false;
    // No auto-claiming for USDC via CCTPV2
    if (isCctp(token)) return false;
    if (loading) return false;
    return true;
  }, [fromChain, token, loading]);

  if (!amount || amount <= 0n) return null;
  if (isConnected && originChainBalanceTooLow) return null;

  return (
    <div className={styles["wrapper"]}>
      <div className={styles.top}>
        <p className={styles.title}>Receive</p>
        <div className={styles.config}>
          <BridgeMode />
          {showSettingIcon && (
            <button className={styles.setting} type="button" onClick={() => setShowAdvancedSettingsModal(true)}>
              <SettingIcon />
            </button>
          )}
        </div>
      </div>

      {loading ? (
        <Skeleton />
      ) : (
        <div className={styles.content}>
          <div className={styles.result}>
            <BridgeTwoLogo
              src1={fromChain?.iconPath ?? ""}
              src2={toChain?.iconPath ?? ""}
              alt1={fromChain?.nativeCurrency.symbol ?? ""}
              alt2={toChain?.nativeCurrency.symbol ?? ""}
            />
            <ReceivedAmount />
          </div>
          <Fees />
        </div>
      )}
      <AdvancedSettings
        isModalOpen={showAdvancedSettingsModal}
        onCloseModal={() => setShowAdvancedSettingsModal(false)}
      />
    </div>
  );
}
