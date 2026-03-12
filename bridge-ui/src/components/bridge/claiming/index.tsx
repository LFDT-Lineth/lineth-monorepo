import { useEffect, useMemo, useState } from "react";

import { useQuery } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useConfig, useConnection } from "wagmi";

import { type DepositWarning, getAdapter } from "@/adapters";
import SettingIcon from "@/assets/icons/setting.svg";
import BridgeTwoLogo from "@/components/bridge/bridge-two-logo";
import Skeleton from "@/components/bridge/claiming/skeleton";
import useFees from "@/hooks/fees/useFees";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { useUiStore } from "@/stores/uiStore";
import { ChainLayer, ClaimType } from "@/types";

import BridgeMode from "./bridge-mode";
import styles from "./claiming.module.scss";
import EstimatedTime from "./estimated-time";
import WithFees from "./fees/with-fees";
import ManualClaim from "./manual-claim";
import ReceivedAmount from "./received-amount";

const AdvancedSettings = dynamic(() => import("@/components/bridge/modal/advanced-settings"), {
  ssr: false,
});

export default function Claiming() {
  const { isConnected } = useConnection();
  const wagmiConfig = useConfig();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const [loading, setLoading] = useState<boolean>(false);
  const [showAdvancedSettingsModal, setShowAdvancedSettingsModal] = useState<boolean>(false);

  const amount = useFormStore((state) => state.amount);
  const token = useFormStore((state) => state.token);
  const selectedMode = useFormStore((state) => state.selectedMode);

  const { hasInsufficientFunds, isLoading: isFeesLoading, effectiveClaimType } = useFees();

  const adapter = getAdapter(token, fromChain, toChain);
  const setHideNoFeesPill = useUiStore((s) => s.setHideNoFeesPill);

  const { data: depositWarnings, isLoading: isWarningsLoading } = useQuery<DepositWarning[] | undefined>({
    queryKey: ["depositWarnings", adapter?.id, fromChain.id, toChain.id, token.symbol, amount?.toString()],
    queryFn: () => adapter!.getDepositWarnings!({ token, fromChain, toChain, amount: amount ?? 0n, wagmiConfig }),
    enabled: !!adapter?.getDepositWarnings && !!amount && amount > 0n,
    staleTime: 30_000,
  });

  const warningLoading = isWarningsLoading && !!adapter?.getDepositWarnings && !!amount && amount > 0n;

  useEffect(() => {
    setLoading(true);
    const timeout = setTimeout(() => {
      setLoading(false);
    }, 1000);

    return () => clearTimeout(timeout);
  }, [amount]);

  useEffect(() => {
    const effectiveMode = selectedMode ?? adapter?.defaultMode;
    const hasProtocolFee = !!adapter?.modes && effectiveMode !== adapter?.defaultMode;
    setHideNoFeesPill(hasProtocolFee);

    return () => setHideNoFeesPill(false);
  }, [selectedMode, adapter, setHideNoFeesPill]);

  const showSettingIcon = useMemo(() => {
    if (fromChain.layer === ChainLayer.L2) return false;
    if (!adapter?.hasAdvancedSettings) return false;
    return !loading;
  }, [fromChain, adapter, loading]);

  if (!amount || amount <= 0n) return null;
  if (isConnected && hasInsufficientFunds) return null;

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

      {loading || isFeesLoading || warningLoading ? (
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
          <div className={styles.estimate}>
            <WithFees iconPath={fromChain.iconPath} />
            <EstimatedTime />
            {effectiveClaimType === ClaimType.MANUAL && <ManualClaim />}
          </div>
        </div>
      )}
      {depositWarnings?.map((warning, i) => (
        <p key={i} className={styles.warning}>
          {warning.text}
          {warning.link && (
            <>
              {" "}
              <Link href={warning.link.url} target="_blank" rel="noopener noreferrer">
                {warning.link.label}
              </Link>
            </>
          )}
        </p>
      ))}
      {showAdvancedSettingsModal && (
        <AdvancedSettings
          isModalOpen={showAdvancedSettingsModal}
          onCloseModal={() => setShowAdvancedSettingsModal(false)}
        />
      )}
    </div>
  );
}
