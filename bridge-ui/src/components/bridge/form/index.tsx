import { useEffect, useState } from "react";
import { useAccount } from "wagmi";
import { useDynamicEvents } from "@/lib/dynamic";
import FaqHelp from "@/components/bridge/faq-help";
import TokenList from "@/components/bridge/token-list";
import { Amount } from "@/components/bridge/amount";
import SwapChain from "@/components/bridge/swap-chain";
import FromChain from "@/components/bridge/from-chain";
import ToChain from "@/components/bridge/to-chain";
import Claiming from "@/components/bridge/claiming";
import styles from "./bridge-form.module.scss";
import { Submit } from "@/components/bridge/submit";
import TransactionPaperIcon from "@/assets/icons/transaction-paper.svg";
import Setting from "@/components/setting";
import { DestinationAddress } from "../destination-address";
import Button from "../../ui/button";
import { useChainStore, useFormStore, useNativeBridgeNavigationStore } from "@/stores";
import { useTokenBalance } from "@/hooks";
import { ChainLayer, ClaimType } from "@/types";

export default function BridgeForm() {
  const [isDestinationAddressOpen, setIsDestinationAddressOpen] = useState(false);
  const setIsTransactionHistoryOpen = useNativeBridgeNavigationStore.useSetIsTransactionHistoryOpen();
  const setIsBridgeOpen = useNativeBridgeNavigationStore.useSetIsBridgeOpen();

  const { address } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const setRecipient = useFormStore((state) => state.setRecipient);
  const setBalance = useFormStore((state) => state.setBalance);
  const setClaim = useFormStore((state) => state.setClaim);
  const resetForm = useFormStore((state) => state.resetForm);

  const { balance, refetch } = useTokenBalance(token);

  useEffect(() => {
    refetch();
  }, [refetch, token]);

  useDynamicEvents("logout", async () => {
    resetForm();
    setIsDestinationAddressOpen(false);
  });

  useEffect(() => {
    setBalance(balance);
    if (address) {
      setRecipient(address);
    }

    if (fromChain.layer === ChainLayer.L2) {
      setClaim(ClaimType.MANUAL);
    }
  }, [balance, address, fromChain.layer, setBalance, setRecipient, setClaim]);

  return (
    <>
      <div className={styles["form-wrapper"]}>
        <div className={styles.headline}>
          <div className={styles["action"]}>
            <Button
              className={styles["transaction-button"]}
              variant="link"
              onClick={() => {
                setIsBridgeOpen(false);
                setIsTransactionHistoryOpen(true);
              }}
              data-testid="native-bridge-transaction-history-icon"
            >
              <TransactionPaperIcon className={styles["transaction-icon"]} />
            </Button>
            <Setting data-testid="native-bridge-form-settings-icon" />
          </div>
        </div>
        <div className={styles["content"]}>
          <div className={styles["exchange"]}>
            <FromChain />
            <div className={styles["swap-chain-container"]}>
              <SwapChain />
            </div>
            <ToChain />
          </div>
          <div className={styles["amount-wrapper"]}>
            <Amount />
            <div className={styles["right"]}>
              <TokenList />
            </div>
          </div>
          <Claiming />
          {isDestinationAddressOpen && (
            <div className={styles["destination-address-wrapper"]}>
              <DestinationAddress />
            </div>
          )}
          <div className={styles["connect-btn-wrapper"]}>
            <Submit
              isDestinationAddressOpen={isDestinationAddressOpen}
              setIsDestinationAddressOpen={() => setIsDestinationAddressOpen((prev) => !prev)}
            />
          </div>
          <FaqHelp isMobile />
        </div>
      </div>
      <FaqHelp />
    </>
  );
}
