import { useEffect, useMemo, useState } from "react";

import dynamic from "next/dynamic";
import { useAccount } from "wagmi";

import SettingIcon from "@/assets/icons/setting.svg";
import BridgeTwoLogo from "@/components/bridge/bridge-two-logo";
import Skeleton from "@/components/bridge/claiming/skeleton";
import { useChainStore, useFormStore } from "@/stores";
import { BridgeProvider, CCTPMode, ChainLayer } from "@/types";
import { isCctp } from "@/utils";

import BridgeMode from "./bridge-mode";
import styles from "./claiming.module.scss";
import Fees from "./fees";
import ReceivedAmount from "./received-amount";

const AdvancedSettings = dynamic(() => import("@/components/bridge/modal/advanced-settings"), {
  ssr: false,
});

export default function Claiming() {
  const { isConnected } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const [loading, setLoading] = useState<boolean>(false);
  const [showAdvancedSettingsModal, setShowAdvancedSettingsModal] = useState<boolean>(false);

  const amount = useFormStore((state) => state.amount);
  const balance = useFormStore((state) => state.balance);
  const token = useFormStore((state) => state.token);
  const cctpMode = useFormStore((state) => state.cctpMode);
  const originChainBalanceTooLow = amount && balance < amount;

  useEffect(() => {
    setLoading(true);
    const timeout = setTimeout(() => {
      setLoading(false);
    }, 1000);

    return () => clearTimeout(timeout);
  }, [amount]);

  useEffect(() => {
    const noFeePill = document.getElementById("no-fees-pill");
    if (!noFeePill) return;

    noFeePill.style.display =
      token.bridgeProvider === BridgeProvider.CCTP && cctpMode === CCTPMode.FAST ? "none" : "block";
  }, [cctpMode, token.bridgeProvider]);

  // Do not allow user to go to AdvancedSettings modal, when they have no choice of ClaimType anyway
  const showSettingIcon = useMemo(() => {
    if (fromChain.layer === ChainLayer.L2) return false;
    // No auto-claiming for USDC via CCTPV2
    if (isCctp(token)) return false;
    return !loading;
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
              src1={token?.image ?? ""}
              src2={toChain?.iconPath ?? ""}
              alt1={token?.symbol ?? ""}
              alt2={toChain?.nativeCurrency.symbol ?? ""}
            />
            <ReceivedAmount />
          </div>
          <Fees />
        </div>
      )}
      {showAdvancedSettingsModal && (
        <AdvancedSettings
          isModalOpen={showAdvancedSettingsModal}
          onCloseModal={() => setShowAdvancedSettingsModal(false)}
        />
      )}
    </div>
  );
}
