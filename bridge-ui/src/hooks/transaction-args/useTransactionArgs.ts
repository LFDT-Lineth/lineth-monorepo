import { QueryObserverResult, RefetchOptions } from "@tanstack/react-query";
import { ReadContractErrorType } from "@wagmi/core";
import { useConnection } from "wagmi";

import { SupportedChainIds } from "@/types";

import useAllowance from "../useAllowance";
import useDepositForBurnTxArgs from "./cctp/useDepositForBurnTxArgs";
import useApproveTxArgs from "./useApproveTxArgs";
import useERC20BridgeTxArgs from "./useERC20BridgeTxArgs";
import useEthBridgeTxArgs from "./useEthBridgeTxArgs";

type TransactionArgs =
  | {
      args: {
        to: `0x${string}`;
        data: `0x${string}`;
        value: bigint;
        chainId: SupportedChainIds;
      };
      type: string;
      refetchAllowance?: (options?: RefetchOptions) => Promise<QueryObserverResult<bigint, ReadContractErrorType>>;
    }
  | undefined;

const useTransactionArgs = (): TransactionArgs => {
  const { isConnected } = useConnection();
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
