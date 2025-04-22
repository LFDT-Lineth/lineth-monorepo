import { useMemo, useEffect } from "react";
import { Address } from "viem";
import useFeeData from "./useFeeData";
import useMessageNumber from "../useMessageNumber";
import useERC20BridgingGasUsed from "./useERC20BridgingGasUsed";
import useEthBridgingGasUsed from "./useEthBridgingGasUsed";
import { useFormStore, useChainStore } from "@/stores";
import { Token, ClaimType } from "@/types";
import { isEth, isUndefined } from "@/utils";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER, MAX_POSTMAN_SPONSOR_GAS_LIMIT } from "@/constants";

type UseBridgingFeeProps = {
  isConnected: boolean;
  token: Token;
  account?: Address;
  recipient: Address;
  amount: bigint;
  claimingType: ClaimType;
};

const useBridgingFee = ({ isConnected, account, token, claimingType, amount, recipient }: UseBridgingFeeProps) => {
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const setBridgingFees = useFormStore((state) => state.setBridgingFees);
  const setClaim = useFormStore((state) => state.setClaim);

  const { feeData } = useFeeData(toChain.id);
  const nextMessageNumber = useMessageNumber({ fromChain, claimingType });

  const fromAddress = isConnected ? account : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;
  const toAddress = isConnected ? recipient : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;

  const eth = useEthBridgingGasUsed({
    account: fromAddress,
    fromChain,
    toChain,
    nextMessageNumber,
    amount,
    recipient: toAddress,
    token,
    claimingType,
  });

  const erc20 = useERC20BridgingGasUsed({
    account: fromAddress,
    token,
    fromChain,
    toChain,
    nextMessageNumber,
    amount,
    recipient: toAddress,
    claimingType,
  });

  const isError = eth.isError || erc20.isError;
  const isLoading = eth.isLoading || erc20.isLoading;
  const gasLimit = isEth(token) ? eth.data : erc20.data;

  const computedBridgingFees = useMemo(() => {
    // Highest priority claim type, if L2->L1 or USDC, do not enable any path to other claim types.
    if (claimingType === ClaimType.MANUAL) {
      return 0n;
    }
    if (isLoading || isError || isUndefined(gasLimit) || isUndefined(feeData)) {
      return null;
    }

    // Computation for AUTO_SPONSORED, i.e. sponsored by the Postman
    const bridgingGasUsedWithSurplus = gasLimit + fromChain.gasLimitSurplus;
    if (bridgingGasUsedWithSurplus < MAX_POSTMAN_SPONSOR_GAS_LIMIT) {
      setClaim(ClaimType.AUTO_SPONSORED);
      return 0n;
    }

    // Computation for ClaimType.AUTO_PAID
    setClaim(ClaimType.AUTO_PAID);
    return feeData * bridgingGasUsedWithSurplus * fromChain.profitMargin;
  }, [
    isLoading,
    isError,
    gasLimit,
    feeData,
    claimingType,
    fromChain.gasLimitSurplus,
    fromChain.profitMargin,
    setClaim,
  ]);

  useEffect(() => {
    if (computedBridgingFees !== null) {
      setBridgingFees(computedBridgingFees);
    }
  }, [computedBridgingFees, setBridgingFees]);

  return { bridgingFees: computedBridgingFees, isLoading };
};

export default useBridgingFee;
