"use client";

import { useContext, useEffect, useMemo, useState } from "react";
import { useAccount, useWaitForTransactionReceipt } from "wagmi";
import { BridgeExternal } from "./BridgeExternal";
import { FromChain } from "./FromChain";
import { Balance } from "./Balance";
import { Amount } from "./form/Amount";
import SwapIcon from "@/assets/icons/swap.svg";
import { ToChain } from "./ToChain";
import { ReceivedAmount } from "./ReceivedAmount";
import { Recipient } from "./form/Recipient";
import { ClaimingType } from "./form/ClaimingType";
import { Fees } from "./fees";
import { Submit } from "./form/Submit";
import { useFormContext } from "react-hook-form";
import { BridgeForm, Transaction } from "@/models";
import { useChainStore } from "@/stores/chainStore";
import { NetworkLayer, TokenType } from "@/config";
import { useBridge, useSwitchNetwork, useFetchHistory } from "@/hooks";
import TokenList from "./TokenList";
import { toast } from "react-toastify";
import { ERC20Stepper } from "./ERC20Stepper";
import ConnectButton from "../ConnectButton";
import { cn } from "@/utils/cn";
import { useReceivedAmount } from "@/hooks/useReceivedAmount";
import { ModalContext } from "@/contexts/modal.context";
import TransactionConfirmationModal from "./modals/TransactionConfirmationModal";

const Bridge = () => {
  const [waitingTransaction, setWaitingTransaction] = useState<Transaction | undefined>();
  const { handleShow, handleClose } = useContext(ModalContext);

  const fromChain = useChainStore((state) => state.fromChain);
  const token = useChainStore((state) => state.token);
  const networkLayer = useChainStore((state) => state.networkLayer);
  const switchChainInStore = useChainStore((state) => state.switchChain);

  const { handleSubmit, watch, reset, setValue } = useFormContext<BridgeForm>();

  const [amount, bridgingAllowed, claim] = watch(["amount", "bridgingAllowed", "claim"]);

  const enoughAllowance = useMemo(() => bridgingAllowed || token?.type === TokenType.ETH, [bridgingAllowed, token]);

  const { totalReceived, fees } = useReceivedAmount({
    amount,
    enoughAllowance,
    claim,
  });

  const { fetchHistory } = useFetchHistory();

  const { isConnected, address } = useAccount();

  const {
    isLoading: isWaitingLoading,
    isSuccess: isWaitingSuccess,
    isError: isWaitingError,
  } = useWaitForTransactionReceipt({
    hash: waitingTransaction?.txHash,
    chainId: waitingTransaction?.chainId,
  });
  const { switchChain } = useSwitchNetwork(fromChain?.id);
  const { hash, isLoading, bridge, error: bridgeError } = useBridge();

  // Set tx hash to trigger useWaitForTransaction
  useEffect(() => {
    if (hash) {
      setWaitingTransaction({
        txHash: hash,
        chainId: fromChain?.id,
        name: fromChain?.name,
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hash]);

  useEffect(() => {
    setValue("gasFees", fees.transactionFeeInWei);
  }, [fees.transactionFeeInWei, setValue]);

  // Clear tx waiting when changing account
  useEffect(() => {
    setWaitingTransaction(undefined);
  }, [address]);

  useEffect(() => {
    if (isWaitingSuccess && waitingTransaction) {
      handleShow(<TransactionConfirmationModal handleClose={handleClose} />, {
        showCloseButton: true,
      });
      setWaitingTransaction(undefined);
      fetchHistory();
      reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isWaitingSuccess, waitingTransaction]);

  useEffect(() => {
    if (isWaitingError) {
      toast.error("Token bridging failed.");
      setWaitingTransaction(undefined);
    }
  }, [isWaitingError]);

  useEffect(() => {
    if (bridgeError?.displayInToast) {
      toast.error(
        <>
          {bridgeError.message}
          <a href={bridgeError.link} target="_blank" className="ml-1 underline">
            here
          </a>
        </>,
        {
          autoClose: false,
          draggable: false,
          closeOnClick: false,
          style: { width: 500, marginLeft: -90 },
        },
      );
    }
  }, [bridgeError]);

  // Click on approve
  const onSubmit = async (data: BridgeForm) => {
    if (isLoading || isWaitingLoading) return;
    await switchChain();
    bridge(data.amount, data.minFees, data.recipient);
  };

  return (
    <>
      <form onSubmit={handleSubmit(onSubmit)}>
        <div className="min-w-min max-w-lg rounded-lg bg-cardBg p-6 shadow-lg sm:p-4">
          <div
            className={cn({
              "opacity-40 pointer-events-none": !isConnected,
            })}
          >
            <FromChain />

            <div className="mb-8">
              <div className="grid grid-flow-col items-center gap-2 rounded-lg bg-backgroundColor p-3">
                <div className="grid grid-flow-row gap-2">
                  <TokenList />
                  <Balance />
                </div>
                <div className="grid grid-flow-row">
                  <Amount />
                </div>
              </div>
            </div>

            <div className="divider my-6 flex justify-center">
              <button
                className="btn btn-circle w-fit border-none bg-transparent shadow-none transition-transform duration-200 hover:border-none hover:bg-transparent hover:shadow-none"
                onClick={(e) => {
                  e.preventDefault();
                  e.currentTarget.classList.toggle("rotate-180");
                  switchChainInStore();
                  reset();
                }}
              >
                <SwapIcon />
              </button>
            </div>

            <ToChain />
            <div className="mb-4">
              <ReceivedAmount receivedAmount={totalReceived} />
            </div>

            <div className="mb-4">
              <Recipient />
            </div>

            <div className="mb-7">
              <ClaimingType />
            </div>

            <div className="mb-7">{isConnected && <Fees totalReceived={totalReceived} fees={fees} />}</div>

            <div className="text-center">
              {isConnected && <Submit isLoading={isLoading} isWaitingLoading={isWaitingLoading} />}
            </div>

            <div className="mt-4">{isConnected && <BridgeExternal />}</div>
          </div>
          {!isConnected && <ConnectButton fullWidth />}
        </div>
      </form>
      {isConnected && token && networkLayer !== NetworkLayer.UNKNOWN && token[networkLayer] && (
        <div className="mt-4 px-2">
          <ERC20Stepper />
        </div>
      )}
    </>
  );
};

export default Bridge;
