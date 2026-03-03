import { Address, maxUint256 } from "viem";
import { useEstimateGas } from "wagmi";

import { getAdapter } from "@/adapters";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER } from "@/constants/general";
import { useChainStore } from "@/stores/chainStore";
import { Chain, Token } from "@/types";
import { isUndefined } from "@/utils/misc";

import useFeeData from "./useFeeData";
import useTransactionSteps from "../transaction-args/useTransactionSteps";

type UseGasFeesProps = {
  token: Token;
  address?: Address;
  fromChain: Chain;
  amount: bigint;
  isConnected: boolean;
};

const useGasFees = ({ address, amount, fromChain, isConnected, token }: UseGasFeesProps) => {
  const toChain = useChainStore.useToChain();
  const { feeData } = useFeeData(fromChain.id);
  const transactionArgs = useTransactionSteps();

  const adapter = getAdapter(token, fromChain, toChain);
  const fallbackGasLimit = adapter?.getFallbackGasLimit?.(token);

  const fromAddress = isConnected ? address : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;
  const canEstimateDisconnected = fallbackGasLimit === undefined;

  const {
    data: estimatedGas,
    isError,
    isLoading,
    refetch,
  } = useEstimateGas({
    chainId: transactionArgs?.args.chainId,
    account: fromAddress,
    value: transactionArgs?.args.value,
    to: transactionArgs?.args.to,
    data: transactionArgs?.args.data,
    ...(!isConnected ? { stateOverride: [{ address: fromAddress!, balance: maxUint256 }] } : {}),
    query: {
      enabled: !!transactionArgs && amount > 0n && !!fromAddress && (isConnected || canEstimateDisconnected),
    },
  });

  if (isLoading) {
    return null;
  }

  let gasFees: bigint | null = null;
  if (!isUndefined(feeData)) {
    if (estimatedGas) {
      gasFees = estimatedGas * feeData;
    } else if (!isConnected && fallbackGasLimit) {
      gasFees = fallbackGasLimit * feeData;
    }
  }

  return {
    gasFees,
    isError: isConnected && isError,
    isLoading,
    refetch,
  };
};

export default useGasFees;
