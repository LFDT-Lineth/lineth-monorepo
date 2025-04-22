import { useMemo, useCallback } from "react";
import { useAccount } from "wagmi";
import { formatEther, zeroAddress } from "viem";
import useGasFees from "./useGasFees";
import useMinimumFee from "./useMinimumFee";
import useBridgingFee from "./useBridgingFee";
import useTokenPrices from "../useTokenPrices";
import { useFormStore, useChainStore } from "@/stores";
import { ClaimType } from "@/types";
import { isZero, isUndefined } from "@/utils";

const useFees = () => {
  const { address, isConnected } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  useMinimumFee();

  const { data: tokenPrices, isLoading: isTokenPricesLoading } = useTokenPrices([zeroAddress], fromChain.id);

  const claim = useFormStore((state) => state.claim);
  const token = useFormStore((state) => state.token);
  const recipient = useFormStore((state) => state.recipient);
  const amount = useFormStore((state) => state.amount);

  const gasFeesResult = useGasFees({
    token,
    isConnected,
    address,
    fromChain,
    amount: amount ?? 0n,
  });

  const { bridgingFees, isLoading: isBridgingFeeLoading } = useBridgingFee({
    isConnected,
    account: address,
    token,
    amount: amount ?? 0n,
    recipient,
    claimingType: claim,
  });

  const getFiatValue = useCallback(
    (fee: bigint) => {
      const zeroAddrLower = zeroAddress.toLowerCase();
      const price = tokenPrices?.[zeroAddrLower];
      if (isZero(price) || isUndefined(price) || price <= 0) return null;
      return Number(formatEther(fee)) * price;
    },
    [tokenPrices],
  );

  const fees = useMemo(() => {
    const feesArray: { name: string; fee: bigint; fiatValue: number | null }[] = [];
    if (gasFeesResult?.gasFees) {
      feesArray.push({
        name: `${fromChain.name} fee`,
        fee: gasFeesResult.gasFees,
        fiatValue: getFiatValue(gasFeesResult.gasFees),
      });

      if (claim === ClaimType.AUTO_SPONSORED) {
        feesArray.push({
          name: `${toChain.name} fee`,
          fee: 0n,
          fiatValue: null,
        });
      }
      if (claim === ClaimType.AUTO_PAID && bridgingFees) {
        feesArray.push({
          name: `${toChain.name} fee`,
          fee: bridgingFees,
          fiatValue: getFiatValue(bridgingFees),
        });
      }
    }
    return feesArray;
  }, [gasFeesResult?.gasFees, fromChain.name, getFiatValue, claim, bridgingFees, toChain.name]);

  const totalFees = useMemo(() => {
    const totalFeeBigInt = fees.reduce<bigint>((acc, fee) => acc + fee.fee, 0n);
    const totalFiat = fees.reduce((acc, fee) => acc + Number(fee.fiatValue ?? 0), 0);
    return {
      fees: totalFeeBigInt,
      fiatValue: totalFiat > 0 ? totalFiat : null,
    };
  }, [fees]);

  const isLoading = gasFeesResult?.isLoading || isBridgingFeeLoading || isTokenPricesLoading;

  return {
    fees,
    total: totalFees,
    isLoading,
  };
};

export default useFees;
