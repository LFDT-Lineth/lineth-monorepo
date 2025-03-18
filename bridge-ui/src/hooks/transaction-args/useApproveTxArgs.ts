import { useMemo } from "react";
import { useAccount } from "wagmi";
import { encodeFunctionData, erc20Abi } from "viem";
import { useFormStore, useChainStore } from "@/stores";
import { isEth } from "@/utils";
import { isCctp } from "@/utils/tokens";
import { CCTP_TOKEN_MESSENGER } from "@/utils/cctp";

type UseERC20ApproveTxArgsProps = {
  allowance?: bigint;
};

const useApproveTxArgs = ({ allowance }: UseERC20ApproveTxArgsProps) => {
  const { address } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);

  return useMemo(() => {
    if (!address || !fromChain || !token || isEth(token) || !amount || allowance === undefined || allowance >= amount) {
      return;
    }

    const spender = !isCctp(token) ? fromChain.tokenBridgeAddress : CCTP_TOKEN_MESSENGER;

    return {
      type: "approve",
      args: {
        to: token[fromChain.layer],
        data: encodeFunctionData({
          abi: erc20Abi,
          functionName: "approve",
          args: [spender, amount],
        }),
        value: 0n,
        chainId: fromChain.id,
      },
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [address, allowance, amount, fromChain.id, token]);
};

export default useApproveTxArgs;
