import { Address, maxUint256 } from "viem";
import { useEstimateGas } from "wagmi";
import { Chain, Token } from "@/types";
import useFeeData from "./useFeeData";
import { useTransactionArgs } from "../transaction-args";
import { isCctp, isEth, isUndefined } from "@/utils";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER } from "@/constants";

type UseGasFeesProps = {
  token: Token;
  address?: Address;
  fromChain: Chain;
  amount: bigint;
  isConnected: boolean;
};

const BRIDGE_TOKEN_GAS_LIMIT = 133_000n;
const DEPOSIT_FOR_BURN_GAS_LIMIT = 112_409n;

function computeGasFees(token: Token, isConnected: boolean, estimatedGas?: bigint, feeData?: bigint) {
  if (isUndefined(feeData)) return null;

  if (isConnected) {
    return estimatedGas ? estimatedGas * feeData : null;
  }

  if (isEth(token)) {
    return estimatedGas ? estimatedGas * feeData : null;
  }

  const gasLimit = isCctp(token) ? DEPOSIT_FOR_BURN_GAS_LIMIT : BRIDGE_TOKEN_GAS_LIMIT;
  return gasLimit * feeData;
}

const useGasFees = ({ address, amount, fromChain, isConnected, token }: UseGasFeesProps) => {
  const { feeData } = useFeeData(fromChain.id);
  const transactionArgs = useTransactionArgs();

  const fromAddress = isConnected ? address : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;

  const isEnabled = isConnected || isEth(token);

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
      enabled: !!transactionArgs && amount > 0n && !!fromAddress && !!isEnabled,
    },
  });

  if (isLoading) {
    return null;
  }

  return {
    gasFees: computeGasFees(token, isConnected, estimatedGas, feeData),
    isError: isConnected && isError,
    isLoading,
    refetch,
  };
};

export default useGasFees;
