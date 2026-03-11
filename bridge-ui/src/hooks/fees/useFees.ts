import { useMemo, useCallback } from "react";

import { formatEther, zeroAddress } from "viem";
import { useBalance, useConnection } from "wagmi";

import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { ClaimType } from "@/types";
import { isZero, isUndefined } from "@/utils/misc";
import { isEth } from "@/utils/tokens";

import useBridgeFees from "./useBridgeFees";
import useGasFees from "./useGasFees";
import useTokenPrices from "../useTokenPrices";

const useFees = () => {
  const { address, isConnected } = useConnection();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);
  const balance = useFormStore((state) => state.balance);
  const claim = useFormStore((state) => state.claim);

  const { data: tokenPrices, isLoading: isTokenPricesLoading } = useTokenPrices([zeroAddress], fromChain.id);
  const { data: nativeBalance } = useBalance({ address, chainId: fromChain.id });

  const { fees: bridgeFees, isLoading: isBridgeFeesLoading, resolvedClaimType, bridgingFeeLabel } = useBridgeFees();

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
    const rows: { name: string; fee: bigint; fiatValue: number | null }[] = [];

    if (gasFeesResult.gasFees) {
      rows.push({
        name: `${fromChain.name} fee`,
        fee: gasFeesResult.gasFees,
        fiatValue: getFiatValue(gasFeesResult.gasFees),
      });

      if (effectiveClaimType === ClaimType.AUTO_SPONSORED) {
        rows.push({ name: `${toChain.name} fee`, fee: 0n, fiatValue: null });
      }
    }

    // AUTO_PAID bridging fee (e.g. Hyperlane) is shown independently of gas estimation
    // because for such adapters the tx cannot be built until the bridging fee is known,
    // creating a circular dependency that prevents gas estimation from running first.
    if (effectiveClaimType === ClaimType.AUTO_PAID && bridgeFees.bridgingFee) {
      rows.push({
        name: bridgingFeeLabel ?? `${toChain.name} fee`,
        fee: bridgeFees.bridgingFee,
        fiatValue: getFiatValue(bridgeFees.bridgingFee),
      });
    }

    return rows;
  }, [
    gasFeesResult.gasFees,
    fromChain.name,
    getFiatValue,
    effectiveClaimType,
    bridgeFees.bridgingFee,
    toChain.name,
    bridgingFeeLabel,
  ]);

  const total = useMemo(() => {
    const totalFeeBigInt = fees.reduce<bigint>((acc, fee) => acc + fee.fee, 0n);
    const totalFiat = fees.reduce((acc, fee) => acc + Number(fee.fiatValue ?? 0), 0);
    return {
      fees: totalFeeBigInt,
      fiatValue: totalFiat > 0 ? totalFiat : null,
    };
  }, [fees]);

  const isLoading = gasFeesResult.isLoading || isBridgeFeesLoading || isTokenPricesLoading;

  // ethFees: gas cost + bridging fee — always paid in native ETH
  //   (includes minimumFee for L2→L1 native bridge which is MANUAL but still costs ETH)
  // tokenFee: protocol fee — paid in the bridged token (e.g. ERC20)
  const hasInsufficientFunds = useMemo(() => {
    if (!amount || amount <= 0n) return false;
    if (!isConnected || !nativeBalance) {
      return !!(amount && balance < amount);
    }

    const ethFees = (gasFeesResult.gasFees ?? 0n) + bridgeFees.bridgingFee;

    if (isEth(token)) {
      return nativeBalance.value < amount + ethFees;
    }

    const tokenFee = bridgeFees.protocolFee ?? 0n;
    return nativeBalance.value < ethFees || balance < amount + tokenFee;
  }, [
    isConnected,
    amount,
    token,
    nativeBalance,
    gasFeesResult.gasFees,
    bridgeFees.bridgingFee,
    bridgeFees.protocolFee,
    balance,
  ]);

  return {
    fees,
    total,
    bridgeFees,
    isLoading,
    hasInsufficientFunds,
    effectiveClaimType,
  };
};

export default useFees;
