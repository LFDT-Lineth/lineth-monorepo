import { useCallback, useEffect, useState } from "react";

import { useConnection } from "wagmi";

import { Amount } from "@/components/bridge/amount";
import Claiming from "@/components/bridge/claiming";
import FaqHelp from "@/components/bridge/faq-help";
import FromChain from "@/components/bridge/from-chain";
import { Submit } from "@/components/bridge/submit";
import SwapChain from "@/components/bridge/swap-chain";
import ToChain from "@/components/bridge/to-chain";
import TokenList from "@/components/bridge/token-list";
import Setting from "@/components/setting";
import { useTokenBalance } from "@/hooks";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { useNativeBridgeNavigationStore } from "@/stores/nativeBridgeNavigationStore";
import { ChainLayer, ClaimType } from "@/types";

import styles from "./bridge-form.module.scss";
import Button from "../../ui/button";
import { DestinationAddress } from "../destination-address";

export default function BridgeForm() {
  const [isDestinationAddressOpen, setIsDestinationAddressOpen] = useState(false);
  const setIsTransactionHistoryOpen = useNativeBridgeNavigationStore.useSetIsTransactionHistoryOpen();
  const setIsBridgeOpen = useNativeBridgeNavigationStore.useSetIsBridgeOpen();

  const { isConnected, address } = useConnection();
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const setRecipient = useFormStore((state) => state.setRecipient);
  const setBalance = useFormStore((state) => state.setBalance);
  const setClaim = useFormStore((state) => state.setClaim);
  const resetForm = useFormStore((state) => state.resetForm);

  const { balance, refetch } = useTokenBalance(token);

  const toggleDestinationAddress = useCallback(() => {
    setIsDestinationAddressOpen((prev) => !prev);
  }, []);

  useEffect(() => {
    refetch();
  }, [refetch, token]);

  useEffect(() => {
    if (!isConnected) {
      resetForm();
      setIsDestinationAddressOpen(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isConnected]);

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
              Transaction History
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
              setIsDestinationAddressOpen={toggleDestinationAddress}
            />
          </div>
        </div>
      </div>
      <FaqHelp />
    </>
  );
}
