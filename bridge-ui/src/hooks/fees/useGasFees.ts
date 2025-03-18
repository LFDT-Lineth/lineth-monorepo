import { Address } from "viem";
import { useEstimateGas } from "wagmi";
import { Chain } from "@/types";
import useFeeData from "./useFeeData";
import { useTransactionArgs } from "../transaction-args";

type UseGasFeesProps = {
  address?: Address;
  fromChain: Chain;
  amount: bigint;
};

const useGasFees = ({ address, amount, fromChain }: UseGasFeesProps) => {
  const { feeData } = useFeeData(fromChain.id);
  const transactionArgs = useTransactionArgs();

  const {
    data: estimatedGas,
    isError,
    isLoading,
    refetch,
  } = useEstimateGas({
    chainId: transactionArgs?.args.chainId,
    account: address,
    value: transactionArgs?.args.value,
    to: transactionArgs?.args.to,
    data: transactionArgs?.args.data,
    query: {
      enabled: !!transactionArgs && amount > 0n && !!address,
    },
  });

  if (isLoading) {
    return null;
  }

  return {
    gasFees: estimatedGas && feeData ? estimatedGas * feeData : null,
    isError,
    isLoading,
    refetch,
  };
};

export default useGasFees;
