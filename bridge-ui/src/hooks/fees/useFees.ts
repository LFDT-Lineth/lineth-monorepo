import { useMemo, useCallback } from "react";

import { formatEther, zeroAddress } from "viem";
import { useConnection } from "wagmi";

import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { ClaimType } from "@/types";
import { isZero, isUndefined } from "@/utils/misc";

import useBridgeFees from "./useBridgeFees";
import useGasFees from "./useGasFees";
import useTokenPrices from "../useTokenPrices";

const useFees = () => {
  const { address, isConnected } = useConnection();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const { data: tokenPrices, isLoading: isTokenPricesLoading } = useTokenPrices([zeroAddress], fromChain.id);

  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);
  const claim = useFormStore((state) => state.claim);

  const { fees: bridgeFees, isLoading: isBridgeFeesLoading, resolvedClaimType } = useBridgeFees();
  const effectiveClaimType = resolvedClaimType ?? claim;

  const gasFeesResult = useGasFees({
    token,
    isConnected,
    address,
    fromChain,
    amount: amount ?? 0n,
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

      if (effectiveClaimType === ClaimType.AUTO_SPONSORED) {
        feesArray.push({
          name: `${toChain.name} fee`,
          fee: 0n,
          fiatValue: null,
        });
      }
      if (effectiveClaimType === ClaimType.AUTO_PAID && bridgeFees.bridgingFee) {
        feesArray.push({
          name: `${toChain.name} fee`,
          fee: bridgeFees.bridgingFee,
          fiatValue: getFiatValue(bridgeFees.bridgingFee),
        });
      }
    }
    return feesArray;
  }, [gasFeesResult?.gasFees, fromChain.name, getFiatValue, effectiveClaimType, bridgeFees.bridgingFee, toChain.name]);

  const totalFees = useMemo(() => {
    const totalFeeBigInt = fees.reduce<bigint>((acc, fee) => acc + fee.fee, 0n);
    const totalFiat = fees.reduce((acc, fee) => acc + Number(fee.fiatValue ?? 0), 0);
    return {
      fees: totalFeeBigInt,
      fiatValue: totalFiat > 0 ? totalFiat : null,
    };
  }, [fees]);

  const isLoading = gasFeesResult?.isLoading || isBridgeFeesLoading || isTokenPricesLoading;

  return {
    fees,
    total: totalFees,
    isLoading,
  };
};

export default useFees;
