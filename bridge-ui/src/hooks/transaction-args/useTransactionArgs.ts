import useEthBridgeTxArgs from "./useEthBridgeTxArgs";
import useERC20BridgeTxArgs from "./useERC20BridgeTxArgs";
import useApproveTxArgs from "./useApproveTxArgs";
import useAllowance from "../useAllowance";
import useDepositForBurnTxArgs from "./cctp/useDepositForBurnTxArgs";
import { QueryObserverResult, RefetchOptions } from "@tanstack/react-query";
import { ReadContractErrorType } from "@wagmi/core";
import { useAccount } from "wagmi";

type TransactionArgs =
  | {
      args: {
        to: `0x${string}`;
        data: `0x${string}`;
        value: bigint;
        chainId: 1 | 59144 | 59141 | 11155111;
      };
      type: string;
      refetchAllowance?: (options?: RefetchOptions) => Promise<QueryObserverResult<bigint, ReadContractErrorType>>;
    }
  | undefined;

const useTransactionArgs = (): TransactionArgs => {
  const { isConnected } = useAccount();
  const ethBridgeTxArgs = useEthBridgeTxArgs({ isConnected });
  const { allowance, refetchAllowance } = useAllowance();
  const erc20BridgeTxArgs = useERC20BridgeTxArgs({ allowance, isConnected });
  const erc20ApproveTxArgs = useApproveTxArgs({ isConnected, allowance });
  const cctpDepositForBurnTxArgs = useDepositForBurnTxArgs({ allowance });

  if (isConnected && erc20ApproveTxArgs) {
    return {
      ...erc20ApproveTxArgs,
      refetchAllowance,
    };
  }

  return ethBridgeTxArgs || erc20BridgeTxArgs || cctpDepositForBurnTxArgs;
};

export default useTransactionArgs;
