"use client";

import { useEffect, useState } from "react";
import { useFormContext } from "react-hook-form";
import { useAccount, useWaitForTransactionReceipt } from "wagmi";
import { parseUnits } from "viem";
import { toast } from "react-toastify";
import { useSwitchNetwork, useAllowance, useApprove } from "@/hooks";
import { Transaction } from "@/models";
import { useChainStore } from "@/stores/chainStore";
import { cn } from "@/utils/cn";

export type BridgeForm = {
  amount: string;
  balance: string;
  submit: string;
};

export default function Approve() {
  const [waitingTransaction, setWaitingTransaction] = useState<Transaction | undefined>();

  // Form
  const { getValues, setValue, watch } = useFormContext();
  const [watchAmount, watchBalance] = watch(["amount", "balance"]);

  // Context
  const token = useChainStore((state) => state.token);
  const fromChain = useChainStore((state) => state.fromChain);
  const tokenBridgeAddress = useChainStore((state) => state.tokenBridgeAddress);

  const { switchChain } = useSwitchNetwork(fromChain?.id);
  const { allowance, refetchAllowance } = useAllowance();
  const { hash: newTxHash, setHash, writeApprove, isLoading: isApprovalLoading } = useApprove();

  // Wagmi
  const { address } = useAccount();

  const hasInsufficientBalance =
    watchAmount && token && parseUnits(watchAmount, token.decimals) > parseUnits(watchBalance, token.decimals);

  const {
    isLoading: isWaitingLoading,
    isSuccess: isWaitingSuccess,
    isError: isWaitingError,
  } = useWaitForTransactionReceipt({
    hash: waitingTransaction?.txHash,
    chainId: waitingTransaction?.chainId,
  });

  // Set form allowance
  useEffect(() => {
    setValue("allowance", allowance);
  }, [allowance, setValue]);

  // Set tx hash to trigger useWaitForTransaction
  useEffect(() => {
    newTxHash &&
      setWaitingTransaction({
        txHash: newTxHash,
        chainId: fromChain?.id,
        name: fromChain?.name,
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [newTxHash]);

  // Clear tx waiting when changing account
  useEffect(() => {
    setWaitingTransaction(undefined);
  }, [address]);

  // Refresh allowance after successful tx
  useEffect(() => {
    refetchAllowance();
    if (isWaitingSuccess) {
      toast.success("Token approval successful!");
      setWaitingTransaction(undefined);
      setHash(null);
    }
  }, [isWaitingSuccess, refetchAllowance, setHash]);

  useEffect(() => {
    if (isWaitingError) {
      toast.error("Token approval failed.");
      setWaitingTransaction(undefined);
      setHash(null);
    }
  }, [isWaitingError, setHash]);

  useEffect(() => {
    if (token && allowance && watchAmount && parseUnits(watchAmount, token.decimals) <= allowance) {
      setValue("bridgingAllowed", true);
    } else {
      setValue("bridgingAllowed", false);
    }
  }, [watchAmount, allowance, token, setValue]);

  const approveHandler = async () => {
    await switchChain();

    if (token) {
      const amount = getValues("amount");
      const amountBigInt = parseUnits(amount, token.decimals);

      writeApprove(amountBigInt, tokenBridgeAddress);
    }
  };

  return (
    <button
      id="approve-btn"
      className={cn("btn btn-primary w-full uppercase rounded-full text-lg font-normal", {
        "btn-disabled":
          token &&
          (isApprovalLoading ||
            isWaitingLoading ||
            !watchAmount ||
            parseUnits(watchAmount, token.decimals) === BigInt(0) ||
            (allowance && parseUnits(watchAmount, token.decimals) <= allowance) ||
            parseUnits(watchAmount, token.decimals) > parseUnits(watchBalance, token.decimals)),
      })}
      type="button"
      onClick={approveHandler}
    >
      {(isApprovalLoading || isWaitingLoading) && <span className="loading loading-spinner"></span>}
      {hasInsufficientBalance ? "Insufficient balance" : "Approve"}
    </button>
  );
}
